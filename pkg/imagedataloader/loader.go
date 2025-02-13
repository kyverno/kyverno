package imagedataloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

var (
	maxReferrersCount = 50
	maxPayloadSize    = int64(10 * 1000 * 1000) // 10 MB
)

type imagedatafetcher struct {
	// TODO: Add caching and prefetching

	lister         k8scorev1.SecretInterface
	defaultOptions []remote.Option
}

type Fetcher interface {
	FetchImageData(ctx context.Context, image string, options ...Option) (*ImageData, error)
}

func New(lister k8scorev1.SecretInterface, opts ...Option) (*imagedatafetcher, error) {
	remoteOpts, err := makeDefaultOpts(lister, opts...)
	if err != nil {
		return nil, err
	}

	return &imagedatafetcher{
		lister:         lister,
		defaultOptions: remoteOpts,
	}, nil
}

func (i *imagedatafetcher) FetchImageData(ctx context.Context, image string, options ...Option) (*ImageData, error) {
	img := ImageData{
		Image:         image,
		referrersData: make(map[string]referrerData),
	}

	var err error
	img.remoteOpts, err = i.remoteOptions(ctx, i.lister, options...)
	if err != nil {
		return nil, err
	}

	img.nameOpts = nameOptions(options...)
	ref, err := name.ParseReference(image, img.nameOpts...)
	if err != nil {
		return nil, err
	}

	img.nameRef = ref
	img.Registry = ref.Context().RegistryStr()
	img.Repository = ref.Context().RepositoryStr()

	if _, ok := ref.(name.Tag); ok {
		img.Tag = ref.Identifier()
	} else {
		img.Digest = ref.Identifier()
	}

	remoteImg, err := remote.Image(ref, img.remoteOpts...)
	if err != nil {
		return nil, err
	}

	img.Manifest, err = remoteImg.Manifest()
	if err != nil {
		return nil, err
	}

	img.ConfigData, err = remoteImg.ConfigFile()
	if err != nil {
		return nil, err
	}

	desc, err := remote.Get(ref, img.remoteOpts...)
	if err != nil {
		return nil, err
	}
	img.desc = desc

	if len(img.Digest) == 0 {
		img.Digest = desc.Digest.String()
	}

	if len(img.Tag) > 0 {
		img.ResolvedImage = fmt.Sprintf("%s:%s@%s", ref.Context().Name(), img.Tag, img.Digest)
	} else {
		img.ResolvedImage = fmt.Sprintf("%s@%s", ref.Context().Name(), img.Digest)
	}

	// error returned means no image index
	imgIndex, _ := desc.ImageIndex()
	if imgIndex != nil {
		imgIndexBytes, err := imgIndex.RawManifest()
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(imgIndexBytes, &img.ImageIndex); err != nil {
			return nil, err
		}
	}

	return &img, nil
}

func (i *imagedatafetcher) remoteOptions(ctx context.Context, lister k8scorev1.SecretInterface, options ...Option) ([]remote.Option, error) {
	var opts []remote.Option
	opts = append(opts, i.defaultOptions...)

	authOpts, err := makeAuthOptions(lister, options...)
	if err != nil {
		return nil, err
	}

	opts = append(opts, authOpts...)
	opts = append(opts, remote.WithContext(ctx))

	return opts, nil
}

type ImageData struct {
	remoteOpts []remote.Option
	nameOpts   []name.Option

	Image         string            `json:"image,omitempty"`
	ResolvedImage string            `json:"resolvedImage,omitempty"`
	Registry      string            `json:"registry,omitempty"`
	Repository    string            `json:"repository,omitempty"`
	Tag           string            `json:"tag,omitempty"`
	Digest        string            `json:"digest,omitempty"`
	ImageIndex    interface{}       `json:"imageIndex,omitempty"`
	Manifest      *gcrv1.Manifest   `json:"manifest,omitempty"`
	ConfigData    *gcrv1.ConfigFile `json:"config,omitempty"`

	nameRef           name.Reference
	desc              *remote.Descriptor
	referrersManifest *gcrv1.IndexManifest
	referrersData     map[string]referrerData
	verifiedReferrers []gcrv1.Descriptor
}

type referrerData struct {
	layerDescriptor *gcrv1.Descriptor
	data            []byte
}

func (i *ImageData) FetchReference(identifier string) (ocispec.Descriptor, error) {
	if identifier == i.Digest {
		return GCRtoOCISpecDesc(i.desc.Descriptor), nil
	}

	d, err := remote.Head(i.nameRef.Context().Digest(identifier), i.remoteOpts...)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	return GCRtoOCISpecDesc(*d), nil
}

func (i *ImageData) WithDigest(digest string) string {
	return i.nameRef.Context().Digest(digest).String()
}

func (i *ImageData) loadReferrers() error {
	if i.referrersManifest != nil {
		return nil
	}

	referrers, err := remote.Referrers(i.nameRef.Context().Digest(i.Digest), i.remoteOpts...)
	if err != nil {
		return err
	}

	referrersDescs, err := referrers.IndexManifest()
	if err != nil {
		return err
	}

	// This check ensures that the manifest does not have an abnormal amount of referrers attached to it to protect against compromised images
	if len(referrersDescs.Manifests) > maxReferrersCount {
		return fmt.Errorf("failed to fetch referrers: to many referrers found, max limit is %d", maxReferrersCount)
	}

	i.referrersManifest = referrersDescs
	return nil
}

func (i *ImageData) FetchRefererrs(artifactType string) ([]gcrv1.Descriptor, error) {
	if err := i.loadReferrers(); err != nil {
		return nil, err
	}

	refList := make([]gcrv1.Descriptor, 0)
	for _, ref := range i.referrersManifest.Manifests {
		if ref.ArtifactType == artifactType {
			refList = append(refList, ref)
		}
	}

	return refList, nil
}

func (i *ImageData) FetchReferrerData(desc gcrv1.Descriptor) ([]byte, *gcrv1.Descriptor, error) {
	if v, found := i.referrersData[desc.Digest.String()]; found {
		return v.data, v.layerDescriptor, nil
	}

	img, err := remote.Image(i.nameRef.Context().Digest(desc.Digest.String()), i.remoteOpts...)
	if err != nil {
		return nil, nil, err
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, nil, err
	}

	if len(layers) != 1 {
		return nil, nil, fmt.Errorf("invalid referrer descriptor, must have only one layer")
	}
	layer := layers[0]

	size, err := layer.Size()
	if err != nil {
		return nil, nil, err
	}
	digest, err := layer.Digest()
	if err != nil {
		return nil, nil, err
	}
	mediaType, err := layer.MediaType()
	if err != nil {
		return nil, nil, err
	}

	layerDesc := &gcrv1.Descriptor{
		MediaType: mediaType,
		Digest:    digest,
		Size:      size,
	}

	reader, err := layer.Uncompressed()
	if err != nil {
		return nil, nil, err
	}

	b, err := io.ReadAll(io.LimitReader(reader, maxPayloadSize))

	i.referrersData[desc.Digest.String()] = referrerData{
		data:            b,
		layerDescriptor: layerDesc,
	}
	return b, layerDesc, err
}

func (i *ImageData) AddVerifiedReferrer(desc gcrv1.Descriptor) {
	if i.verifiedReferrers == nil {
		i.verifiedReferrers = make([]gcrv1.Descriptor, 0)
	}

	i.verifiedReferrers = append(i.verifiedReferrers, desc)
}

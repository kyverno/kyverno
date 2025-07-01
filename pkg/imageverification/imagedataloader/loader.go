package imagedataloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	maxReferrersCount = 50
	maxPayloadSize    = int64(10 * 1000 * 1000) // 10 MB
)

type Fetcher interface {
	FetchImageData(ctx context.Context, image string, options ...Option) (*ImageData, error)
}

type imagedatafetcher struct {
	lister         k8scorev1.SecretInterface
	defaultOptions []remote.Option
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
		referrersData: make(map[string]referrerData),
	}

	var err error
	img.remoteOpts, err = i.remoteOptions(ctx, i.lister, options...)
	if err != nil {
		return nil, err
	}

	img.ImageReference, err = ParseImageReference(image, options...)
	if err != nil {
		return nil, err
	}

	img.nameOpts = nameOptions(options...)
	ref, err := name.ParseReference(image, img.nameOpts...)
	if err != nil {
		return nil, err
	}
	img.nameRef = ref

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

type ImageDescriptor struct {
	ImageReference `json:",inline"`
	ImageIndex     any               `json:"imageIndex,omitempty"`
	Manifest       *gcrv1.Manifest   `json:"manifest,omitempty"`
	ConfigData     *gcrv1.ConfigFile `json:"config,omitempty"`
}

type referrerData struct {
	layerDescriptor *gcrv1.Descriptor
	data            []byte
}

type ImageData struct {
	ImageDescriptor        `json:",inline"`
	remoteOpts             []remote.Option
	nameOpts               []name.Option
	nameRef                name.Reference
	desc                   *remote.Descriptor
	referrersManifest      *gcrv1.IndexManifest
	referrersData          map[string]referrerData
	verifiedReferrers      map[string]gcrv1.Descriptor
	verifiedIntotoPayloads map[string][]byte
}

func (i *ImageData) FetchReference(identifier string) (ocispec.Descriptor, error) {
	if identifier == i.Digest {
		return GCRtoOCISpecDesc(i.desc.Descriptor), nil
	} else if i.referrersManifest != nil {
		for _, m := range i.referrersManifest.Manifests {
			if identifier == m.Digest.String() {
				return GCRtoOCISpecDesc(m), nil
			}
		}
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

func (i *ImageData) Data() ImageDescriptor {
	return i.ImageDescriptor
}

func (i *ImageData) RemoteOpts() []remote.Option {
	return i.remoteOpts
}

func (i *ImageData) NameOpts() []name.Option {
	return i.nameOpts
}

func (i *ImageData) NameRef() name.Reference {
	return i.nameRef
}

func (i *ImageData) FetchReferrersForDigest(digest string, artifactType string) ([]gcrv1.Descriptor, error) {
	// If the call is for image referrers, return prefetched referrers
	if digest == i.Digest {
		return i.FetchReferrers(artifactType)
	}

	// this is most likely a call to fetch notary signatures for an attesatation
	idx, err := i.fetchReferrersFromRemote(digest)
	if err != nil {
		return nil, err
	}

	refList := make([]gcrv1.Descriptor, 0)
	for _, ref := range idx.Manifests {
		if ref.ArtifactType == artifactType {
			refList = append(refList, ref)
		}
	}

	return refList, nil
}

func (i *ImageData) FetchReferrers(artifactType string) ([]gcrv1.Descriptor, error) {
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
		i.verifiedReferrers = make(map[string]gcrv1.Descriptor, 0)
	}

	i.verifiedReferrers[desc.ArtifactType] = desc
}

func (i *ImageData) AddVerifiedIntotoPayloads(predicateType string, data []byte) {
	if i.verifiedIntotoPayloads == nil {
		i.verifiedIntotoPayloads = make(map[string][]byte, 0)
	}

	i.verifiedIntotoPayloads[predicateType] = data
}

func (i *ImageData) GetPayload(a v1alpha1.Attestation) (any, error) {
	var b []byte
	if a.IsInToto() {
		var ok bool
		b, ok = i.verifiedIntotoPayloads[a.InToto.Type]
		if !ok {
			// download this using cosign cli
			return nil, fmt.Errorf("intoto attestation payload cannot be fetch before verifying intoto attestation")
		}
	} else if a.IsReferrer() {
		var err error
		if i.verifiedReferrers != nil {
			desc, ok := i.verifiedReferrers[a.Referrer.Type]
			if ok {
				b, _, err = i.FetchReferrerData(desc)
				if err != nil {
					return nil, err
				}
			}
		}

		if len(b) == 0 {
			descs, err := i.FetchReferrers(a.Referrer.Type)
			if err != nil {
				return nil, err
			}
			if len(descs) == 0 {
				return nil, fmt.Errorf("artifact of type %s not found", a.Referrer.Type)
			}

			b, _, err = i.FetchReferrerData(descs[0])
			if err != nil {
				return nil, err
			}
		}
	}

	if len(b) == 0 {
		return nil, fmt.Errorf("could not find attestation %s", a.GetKey())
	}

	var payload any

	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json for attestation %s", a.GetKey())
	}

	return payload, nil
}

func (i *ImageData) loadReferrers() error {
	if i.referrersManifest != nil {
		return nil
	}

	referrersDescs, err := i.fetchReferrersFromRemote(i.Digest)
	if err != nil {
		return err
	}

	i.referrersManifest = referrersDescs
	return nil
}

func (i *ImageData) fetchReferrersFromRemote(digest string) (*gcrv1.IndexManifest, error) {
	referrers, err := remote.Referrers(i.nameRef.Context().Digest(digest), i.remoteOpts...)
	if err != nil {
		return nil, err
	}

	referrersDescs, err := referrers.IndexManifest()
	if err != nil {
		return nil, err
	}

	// This check ensures that the manifest does not have an abnormal amount of referrers attached to it to protect against compromised images
	if len(referrersDescs.Manifests) > maxReferrersCount {
		return nil, fmt.Errorf("failed to fetch referrers: to many referrers found, max limit is %d", maxReferrersCount)
	}

	return referrersDescs, nil
}

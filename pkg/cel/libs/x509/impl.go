package x509

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net"
	"time"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
	"golang.org/x/crypto/cryptobyte"
	cryptobyte_asn1 "golang.org/x/crypto/cryptobyte/asn1"
)

type impl struct {
	types.Adapter
}

type PublicKey struct {
	N string
	E int
}

type CertificateOutput struct {
	Raw                         string
	RawTBSCertificate           string
	RawSubjectPublicKeyInfo     string
	RawSubject                  string
	RawIssuer                   string
	Signature                   string
	PublicKeyAlgorithm          x509.PublicKeyAlgorithm
	PublicKey                   PublicKey
	SerialNumber                int64
	Issuer                      pkix.Name
	NotBefore                   string
	NotAfter                    string
	KeyUsage                    x509.KeyUsage
	Extensions                  []pkix.Extension
	ExtraExtensions             []pkix.Extension
	ExtKeyUsage                 []x509.ExtKeyUsage
	BasicConstraintsValid       bool
	IsCA                        bool
	MaxPathLen                  int
	MaxPathLenZero              bool
	AuthorityKeyId              []byte
	OCSPServer                  []string
	IssuingCertificateURL       []string
	DNSNames                    []string
	EmailAddresses              []string
	IPAddresses                 []net.IP
	PermittedDNSDomainsCritical bool
	PermittedDNSDomains         []string
	ExcludedDNSDomains          []string
	PermittedIPRanges           []*net.IPNet
	ExcludedIPRanges            []*net.IPNet
	PermittedEmailAddresses     []string
	ExcludedEmailAddresses      []string
	PermittedURIDomains         []string
	ExcludedURIDomains          []string
	CRLDistributionPoints       []string
	PolicyIdentifiers           []asn1.ObjectIdentifier
}

func encodeCertificate(cert *x509.Certificate) (map[string]any, error) {
	output := CertificateOutput{
		Raw:                         base64Encode(cert.Raw),
		RawTBSCertificate:           base64Encode(cert.RawTBSCertificate),
		RawSubjectPublicKeyInfo:     base64Encode(cert.RawSubjectPublicKeyInfo),
		RawSubject:                  base64Encode(cert.RawSubject),
		RawIssuer:                   base64Encode(cert.RawIssuer),
		Signature:                   base64Encode(cert.Signature),
		PublicKeyAlgorithm:          cert.PublicKeyAlgorithm,
		PublicKey:                   cert.PublicKey.(PublicKey),
		SerialNumber:                cert.SerialNumber.Int64(),
		Issuer:                      cert.Issuer,
		NotBefore:                   cert.NotBefore.Format(time.RFC3339),
		NotAfter:                    cert.NotAfter.Format(time.RFC3339),
		KeyUsage:                    cert.KeyUsage,
		Extensions:                  cert.Extensions,
		ExtraExtensions:             cert.ExtraExtensions,
		ExtKeyUsage:                 cert.ExtKeyUsage,
		BasicConstraintsValid:       cert.BasicConstraintsValid,
		IsCA:                        cert.IsCA,
		MaxPathLen:                  cert.MaxPathLen,
		MaxPathLenZero:              cert.MaxPathLenZero,
		AuthorityKeyId:              cert.AuthorityKeyId,
		OCSPServer:                  cert.OCSPServer,
		IssuingCertificateURL:       cert.IssuingCertificateURL,
		DNSNames:                    cert.DNSNames,
		EmailAddresses:              cert.EmailAddresses,
		IPAddresses:                 cert.IPAddresses,
		PermittedDNSDomainsCritical: cert.PermittedDNSDomainsCritical,
		PermittedDNSDomains:         cert.PermittedDNSDomains,
		ExcludedDNSDomains:          cert.ExcludedDNSDomains,
		PermittedIPRanges:           cert.PermittedIPRanges,
		ExcludedIPRanges:            cert.ExcludedIPRanges,
		PermittedEmailAddresses:     cert.PermittedEmailAddresses,
		ExcludedEmailAddresses:      cert.ExcludedEmailAddresses,
		PermittedURIDomains:         cert.PermittedURIDomains,
		ExcludedURIDomains:          cert.ExcludedURIDomains,
		CRLDistributionPoints:       cert.CRLDistributionPoints,
		PolicyIdentifiers:           cert.PolicyIdentifiers,
	}

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(output); err != nil {
		return nil, err
	}
	res := map[string]any{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		return nil, err
	}
	return res, nil
}

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func encode[T any](in T) (map[string]any, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(in); err != nil {
		return nil, err
	}
	res := map[string]any{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (c *impl) decode(val ref.Val) ref.Val {
	input, err := utils.ConvertToNative[string](val)
	if err != nil {
		return types.WrapErr(err)
	}

	block, _ := pem.Decode([]byte(input))
	if block == nil {
		return types.WrapErr(errors.New("failed to decode PEM block"))
	}

	switch block.Type {
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return types.WrapErr(err)
		}
		if cert.PublicKeyAlgorithm != x509.RSA {
			return types.WrapErr(errors.New("certificate should use rsa algorithm"))
		}
		pk, err := parseSubjectPublicKeyInfo(cert.RawSubjectPublicKeyInfo)
		if err != nil {
			return types.WrapErr(errors.New("failed to parse subject public key info"))
		}
		cert.PublicKey = PublicKey{
			N: pk.N.String(),
			E: pk.E,
		}
		result, err := encodeCertificate(cert)
		if err != nil {
			return types.WrapErr(err)
		}
		return c.NativeToValue(result)

	case "CERTIFICATE REQUEST":
		csr, err := x509.ParseCertificateRequest(block.Bytes)
		if err != nil {
			return types.WrapErr(err)
		}
		if csr.PublicKeyAlgorithm != x509.RSA {
			return types.WrapErr(errors.New("certificate should use rsa algorithm"))
		}
		pk, err := parseSubjectPublicKeyInfo(csr.RawSubjectPublicKeyInfo)
		if err != nil {
			return types.WrapErr(errors.New("failed to parse subject public key info"))
		}
		csr.PublicKey = PublicKey{
			N: pk.N.String(),
			E: pk.E,
		}
		result, err := encode(csr)
		if err != nil {
			return types.WrapErr(err)
		}
		return c.NativeToValue(result)

	default:
		return types.WrapErr(errors.New("PEM block neither contains a CERTIFICATE or CERTIFICATE REQUEST"))
	}
}

func parseSubjectPublicKeyInfo(data []byte) (*rsa.PublicKey, error) {
	spki := cryptobyte.String(data)
	if !spki.ReadASN1(&spki, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("writing asn.1 element to 'spki' failed")
	}
	var pkAISeq cryptobyte.String
	if !spki.ReadASN1(&pkAISeq, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("writing asn.1 element to 'pkAISeq' failed")
	}
	var spk asn1.BitString
	if !spki.ReadASN1BitString(&spk) {
		return nil, errors.New("writing asn.1 bit string to 'spk' failed")
	}
	kk, err := x509.ParsePKCS1PublicKey(spk.Bytes)
	if err != nil {
		return nil, err
	}
	return kk, nil
}

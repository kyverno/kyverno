/*-
 * Copyright 2018 Square Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package jose

import (
	"fmt"
	"testing"
)

type signWrapper struct {
	pk      *JSONWebKey
	wrapped payloadSigner
	algs    []SignatureAlgorithm
}

var _ = OpaqueSigner(&signWrapper{})

func (sw *signWrapper) Algs() []SignatureAlgorithm {
	return sw.algs
}

func (sw *signWrapper) Public() *JSONWebKey {
	return sw.pk
}

func (sw *signWrapper) SignPayload(payload []byte, alg SignatureAlgorithm) ([]byte, error) {
	sig, err := sw.wrapped.signPayload(payload, alg)
	if err != nil {
		return nil, err
	}
	return sig.Signature, nil
}

type verifyWrapper struct {
	wrapped []payloadVerifier
}

var _ = OpaqueVerifier(&verifyWrapper{})

func (vw *verifyWrapper) VerifyPayload(payload []byte, signature []byte, alg SignatureAlgorithm) error {
	if len(vw.wrapped) == 0 {
		return fmt.Errorf("error: verifier had no keys")
	}
	var err error
	for _, v := range vw.wrapped {
		err = v.verifyPayload(payload, signature, alg)
		if err == nil {
			return nil
		}
	}
	return err
}

type keyEncryptWrapper struct {
	kid     string
	wrapped keyEncrypter
	algs    []KeyAlgorithm
}

var _ = OpaqueKeyEncrypter(&keyEncryptWrapper{})

func (kew *keyEncryptWrapper) KeyID() string {
	return kew.kid
}

func (kew *keyEncryptWrapper) Algs() []KeyAlgorithm {
	return kew.algs
}

func (kew *keyEncryptWrapper) encryptKey(cek []byte, alg KeyAlgorithm) (recipientInfo, error) {
	info, err := kew.wrapped.encryptKey(cek, alg)
	if err != nil {
		return recipientInfo{}, err
	}

	return info, nil
}

type keyDecryptWrapper struct {
	wrapped keyDecrypter
}

var _ = OpaqueKeyDecrypter(&keyDecryptWrapper{})

func (kdw *keyDecryptWrapper) DecryptKey(encryptedKey []byte, header Header) ([]byte, error) {
	rawHeader := rawHeader{}

	err := rawHeader.set(headerKeyID, header.KeyID)
	if err != nil {
		return nil, err
	}
	err = rawHeader.set(headerAlgorithm, header.Algorithm)
	if err != nil {
		return nil, err
	}
	err = rawHeader.set(headerNonce, header.Nonce)
	if err != nil {
		return nil, err
	}
	err = rawHeader.set(headerJWK, header.JSONWebKey)
	if err != nil {
		return nil, err
	}
	for k, v := range header.ExtraHeaders {
		err = rawHeader.set(k, v)
		if err != nil {
			return nil, err
		}
	}

	recipient := &recipientInfo{
		encryptedKey: encryptedKey,
	}

	var generator randomKeyGenerator
	cipher := getContentCipher(rawHeader.getEncryption())
	if cipher != nil {
		generator = randomKeyGenerator{
			size: cipher.keySize(),
		}
	}

	return kdw.wrapped.decryptKey(rawHeader, recipient, generator)
}

func TestRoundtripsJWSOpaque(t *testing.T) {
	sigAlgs := []SignatureAlgorithm{RS256, RS384, RS512, PS256, PS384, PS512, ES256, ES384, ES512, EdDSA}

	serializers := []func(*JSONWebSignature) (string, error){
		func(obj *JSONWebSignature) (string, error) { return obj.CompactSerialize() },
		func(obj *JSONWebSignature) (string, error) { return obj.FullSerialize(), nil },
	}

	corrupter := func(obj *JSONWebSignature) {}

	for _, alg := range sigAlgs {
		signingKey, verificationKey := GenerateSigningTestKey(alg)

		for i, serializer := range serializers {
			sw := makeOpaqueSigner(t, signingKey, alg)
			vw := makeOpaqueVerifier(t, []interface{}{verificationKey}, alg)

			err := RoundtripJWS(alg, serializer, corrupter, sw, verificationKey, "test_nonce")
			if err != nil {
				t.Error(err, alg, i)
			}

			err = RoundtripJWS(alg, serializer, corrupter, signingKey, vw, "test_nonce")
			if err != nil {
				t.Error(err, alg, i)
			}

			err = RoundtripJWS(alg, serializer, corrupter, sw, vw, "test_nonce")
			if err != nil {
				t.Error(err, alg, i)
			}
		}
	}
}

func makeOpaqueSigner(t *testing.T, signingKey interface{}, alg SignatureAlgorithm) *signWrapper {
	ri, err := makeJWSRecipient(alg, signingKey)
	if err != nil {
		t.Fatal(err)
	}
	return &signWrapper{
		wrapped: ri.signer,
		algs:    []SignatureAlgorithm{alg},
		pk:      &JSONWebKey{Key: ri.publicKey()},
	}
}

func makeOpaqueVerifier(t *testing.T, verificationKey []interface{}, alg SignatureAlgorithm) *verifyWrapper {
	var verifiers []payloadVerifier
	for _, vk := range verificationKey {
		verifier, err := newVerifier(vk)
		if err != nil {
			t.Fatal(err)
		}
		verifiers = append(verifiers, verifier)
	}
	return &verifyWrapper{wrapped: verifiers}
}

func makeOpaqueKeyEncrypter(t *testing.T, signingKey interface{}, alg KeyAlgorithm, kid string) *keyEncryptWrapper {
	rki, err := makeJWERecipient(alg, signingKey)
	if err != nil {
		t.Fatal(err, alg)
	}
	return &keyEncryptWrapper{
		wrapped: rki.keyEncrypter,
		algs:    []KeyAlgorithm{alg},
		kid:     kid,
	}
}

func makeOpaqueKeyDecrypter(t *testing.T, decryptionKey interface{}, alg KeyAlgorithm) *keyDecryptWrapper {
	kd, err := newDecrypter(decryptionKey)
	if err != nil {
		t.Fatal(err)
	}

	return &keyDecryptWrapper{
		wrapped: kd,
	}
}

func TestOpaqueSignerKeyRotation(t *testing.T) {

	sigAlgs := []SignatureAlgorithm{RS256, RS384, RS512, PS256, PS384, PS512, ES256, ES384, ES512, EdDSA}

	serializers := []func(*JSONWebSignature) (string, error){
		func(obj *JSONWebSignature) (string, error) { return obj.CompactSerialize() },
		func(obj *JSONWebSignature) (string, error) { return obj.FullSerialize(), nil },
	}

	for _, alg := range sigAlgs {
		for i, serializer := range serializers {
			sk1, pk1 := GenerateSigningTestKey(alg)
			sk2, pk2 := GenerateSigningTestKey(alg)

			sw := makeOpaqueSigner(t, sk1, alg)
			sw.pk.KeyID = "first"
			vw := makeOpaqueVerifier(t, []interface{}{pk1, pk2}, alg)

			signer, err := NewSigner(
				SigningKey{Algorithm: alg, Key: sw},
				&SignerOptions{NonceSource: staticNonceSource("test_nonce")},
			)
			if err != nil {
				t.Fatal(err, alg, i)
			}

			jws1, err := signer.Sign([]byte("foo bar baz"))
			if err != nil {
				t.Fatal(err, alg, i)
			}
			jws1 = rtSerialize(t, serializer, jws1, vw)
			if kid := jws1.Signatures[0].Protected.KeyID; kid != "first" {
				t.Errorf("expected kid %q but got %q", "first", kid)
			}

			swNext := makeOpaqueSigner(t, sk2, alg)
			swNext.pk.KeyID = "next"
			sw.wrapped = swNext.wrapped
			sw.pk = swNext.pk

			jws2, err := signer.Sign([]byte("foo bar baz next"))
			if err != nil {
				t.Error(err, alg, i)
			}
			jws2 = rtSerialize(t, serializer, jws2, vw)
			if kid := jws2.Signatures[0].Protected.KeyID; kid != "next" {
				t.Errorf("expected kid %q but got %q", "next", kid)
			}
		}
	}
}

func rtSerialize(t *testing.T, serializer func(*JSONWebSignature) (string, error), sig *JSONWebSignature, vk interface{}) *JSONWebSignature {
	b, err := serializer(sig)
	if err != nil {
		t.Fatal(err)
	}
	sig, err = ParseSigned(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sig.Verify(vk); err != nil {
		t.Fatal(err)
	}
	return sig
}

func TestOpaqueKeyRoundtripJWE(t *testing.T) {
	keyAlgs := []KeyAlgorithm{
		ECDH_ES_A128KW, ECDH_ES_A192KW, ECDH_ES_A256KW, A128KW, A192KW, A256KW,
		RSA1_5, RSA_OAEP, RSA_OAEP_256, A128GCMKW, A192GCMKW, A256GCMKW,
		PBES2_HS256_A128KW, PBES2_HS384_A192KW, PBES2_HS512_A256KW,
	}
	encAlgs := []ContentEncryption{A128GCM, A192GCM, A256GCM, A128CBC_HS256, A192CBC_HS384, A256CBC_HS512}
	kid := "test-kid"

	serializers := []func(*JSONWebEncryption) (string, error){
		func(obj *JSONWebEncryption) (string, error) { return obj.CompactSerialize() },
		func(obj *JSONWebEncryption) (string, error) { return obj.FullSerialize(), nil },
	}

	for _, alg := range keyAlgs {
		for _, enc := range encAlgs {
			for _, testKey := range generateTestKeys(alg, enc) {
				for _, serializer := range serializers {
					kew := makeOpaqueKeyEncrypter(t, testKey.enc, alg, kid)
					encrypter, err := NewEncrypter(
						enc,
						Recipient{
							Algorithm: alg,
							Key:       kew,
						},
						&EncrypterOptions{},
					)
					if err != nil {
						t.Fatal(err, alg)
					}

					jwe, err := encrypter.Encrypt([]byte("foo bar"))
					if err != nil {
						t.Fatal(err, alg)
					}

					dw := makeOpaqueKeyDecrypter(t, testKey.dec, alg)
					jwe = jweSerialize(t, serializer, jwe, dw)
					if jwe.Header.KeyID != kid {
						t.Errorf("expected jwe kid to equal %s but got %s", kid, jwe.Header.KeyID)
					}

					out, err := jwe.Decrypt(dw)
					if err != nil {
						t.Fatal(err, out)
					}
					if string(out) != "foo bar" {
						t.Errorf("expected decrypted jwe to equal %s but got %s", "foo bar", string(out))
					}
				}
			}
		}
	}
}

func jweSerialize(t *testing.T, serializer func(*JSONWebEncryption) (string, error), jwe *JSONWebEncryption, d OpaqueKeyDecrypter) *JSONWebEncryption {
	b, err := serializer(jwe)
	if err != nil {
		t.Fatal(err)
	}
	jwe, err = ParseEncrypted(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jwe.Decrypt(d); err != nil {
		t.Fatal(err)
	}
	return jwe
}

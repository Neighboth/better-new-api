package apkbrand

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const (
	// apkSigningBlockMagic marks the APK signing block.
	apkSigningBlockMagic = "APK Sig Block 42"
	// apkSignatureSchemeV2BlockID identifies the v2 signature block.
	apkSignatureSchemeV2BlockID = 0x7109871a
	// contentDigestSHA256 is the RSASSA-PKCS1-v1_5 with SHA-256 algorithm id.
	contentDigestSHA256 = 0x0103
	// contentChunkSize is the chunk size used by the APK content digest.
	contentChunkSize = 1024 * 1024
	// minSDKVersion is the lowest Android version the signature is valid for.
	minSDKVersion = 24
	// maxSDKVersion is the highest Android version the signature is valid for.
	maxSDKVersion = math.MaxInt32
)

// signV2 applies an APK Signature Scheme v2 signature to the given archive.
func signV2(archive []byte, signer *Signer) ([]byte, error) {
	eocdOffset, err := findEndOfCentralDirectory(archive)
	if err != nil {
		return nil, err
	}
	cdSize := binary.LittleEndian.Uint32(archive[eocdOffset+12:])
	cdOffset := binary.LittleEndian.Uint32(archive[eocdOffset+16:])
	if int(cdOffset)+int(cdSize) > len(archive) || int(cdOffset) > eocdOffset {
		return nil, errors.New("apkbrand: malformed central directory")
	}

	// The signing block is inserted right before the central directory, therefore the
	// offset of the central directory in the end of central directory record has to be
	// treated as the offset of the signing block while the digest is computed.
	eocd := make([]byte, len(archive)-eocdOffset)
	copy(eocd, archive[eocdOffset:])
	binary.LittleEndian.PutUint32(eocd[16:], cdOffset)

	digest, err := contentDigest(
		archive[:cdOffset],
		archive[cdOffset:eocdOffset],
		eocd,
	)
	if err != nil {
		return nil, err
	}

	signedData, err := buildSignedData(signer, digest)
	if err != nil {
		return nil, err
	}
	signedDataHash := sha256.Sum256(signedData)
	signature, err := rsa.SignPKCS1v15(rand.Reader, signer.PrivateKey, crypto.SHA256, signedDataHash[:])
	if err != nil {
		return nil, err
	}
	publicKey, err := x509.MarshalPKIXPublicKey(&signer.PrivateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	// A signer consists of the signed data, the signatures over it and the public key.
	// The signature sequence holds one length prefixed element per signature, every
	// element carries the algorithm id followed by the length prefixed signature.
	signatureEntry := append(encodeUint32(contentDigestSHA256), encodeLengthPrefixed(signature)...)
	signerBlock := make([]byte, 0, len(signedData)+len(signature)+len(publicKey)+64)
	signerBlock = append(signerBlock, encodeLengthPrefixed(signedData)...)
	signerBlock = append(signerBlock, encodeLengthPrefixed(encodeLengthPrefixed(signatureEntry))...)
	signerBlock = append(signerBlock, encodeLengthPrefixed(publicKey)...)

	// The v2 block holds a length prefixed sequence of length prefixed signer blocks.
	v2Block := encodeLengthPrefixed(encodeLengthPrefixed(signerBlock))

	pairLength := uint64(4 + len(v2Block))
	// The size field covers everything but the leading size field itself, that means
	// the pair, the repeated size field and the magic.
	blockSize := uint64(8 + 4 + len(v2Block) + 8 + len(apkSigningBlockMagic))
	signingBlock := make([]byte, 0, int(blockSize)+8)
	signingBlock = append(signingBlock, encodeUint64(blockSize)...)
	signingBlock = append(signingBlock, encodeUint64(pairLength)...)
	signingBlock = append(signingBlock, encodeUint32(apkSignatureSchemeV2BlockID)...)
	signingBlock = append(signingBlock, v2Block...)
	signingBlock = append(signingBlock, encodeUint64(blockSize)...)
	signingBlock = append(signingBlock, []byte(apkSigningBlockMagic)...)

	out := make([]byte, 0, len(archive)+len(signingBlock))
	out = append(out, archive[:cdOffset]...)
	out = append(out, signingBlock...)
	out = append(out, archive[cdOffset:]...)

	newEOCD := eocdOffset + len(signingBlock)
	binary.LittleEndian.PutUint32(out[newEOCD+16:], cdOffset+uint32(len(signingBlock)))
	return out, nil
}

// buildSignedData assembles the signed data block of a signer.
//
// The block contains the length prefixed sequence of digests, the length prefixed
// sequence of certificates and the length prefixed sequence of additional attributes.
// The attribute sequence is empty, the platform ignores it as well.
func buildSignedData(signer *Signer, digest []byte) ([]byte, error) {
	certDER := signer.Cert.Raw
	digestEntry := append(encodeUint32(contentDigestSHA256), encodeLengthPrefixed(digest)...)
	certEntry := encodeLengthPrefixed(certDER)

	signedData := make([]byte, 0, 1024)
	signedData = append(signedData, encodeLengthPrefixed(encodeLengthPrefixed(digestEntry))...)
	signedData = append(signedData, encodeLengthPrefixed(certEntry)...)
	signedData = append(signedData, encodeUint32(0)...)
	return signedData, nil
}

// contentDigest computes the chunked digest of the protected APK sections.
//
// Every section is split into one megabyte chunks. The digest of a chunk is computed
// over the chunk marker, the chunk length and the chunk content. The final digest is
// computed over the section marker, the number of chunks and the chunk digests.
func contentDigest(sections ...[]byte) ([]byte, error) {
	var chunkDigests [][]byte
	for _, section := range sections {
		for offset := 0; offset < len(section); offset += contentChunkSize {
			end := offset + contentChunkSize
			if end > len(section) {
				end = len(section)
			}
			chunk := section[offset:end]
			hasher := sha256.New()
			hasher.Write([]byte{0xa5})
			hasher.Write(encodeUint32(uint32(len(chunk))))
			hasher.Write(chunk)
			chunkDigests = append(chunkDigests, hasher.Sum(nil))
		}
	}
	if len(chunkDigests) > math.MaxInt32 {
		return nil, errors.New("apkbrand: input too long")
	}

	hasher := sha256.New()
	hasher.Write([]byte{0x5a})
	hasher.Write(encodeUint32(uint32(len(chunkDigests))))
	for _, digest := range chunkDigests {
		hasher.Write(digest)
	}
	return hasher.Sum(nil), nil
}

// encodeLengthPrefixed prefixes the payload with its length.
func encodeLengthPrefixed(payload []byte) []byte {
	out := make([]byte, 0, len(payload)+4)
	out = append(out, encodeUint32(uint32(len(payload)))...)
	return append(out, payload...)
}

// encodeUint32 encodes a little endian uint32.
func encodeUint32(value uint32) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, value)
	return out
}

// encodeUint64 encodes a little endian uint64.
func encodeUint64(value uint64) []byte {
	out := make([]byte, 8)
	binary.LittleEndian.PutUint64(out, value)
	return out
}

// verifySignature is a helper used by the tests to validate a signed archive.
func verifySignature(archive []byte, signer *Signer) error {
	eocdOffset, err := findEndOfCentralDirectory(archive)
	if err != nil {
		return err
	}
	cdOffset := binary.LittleEndian.Uint32(archive[eocdOffset+16:])
	magicOffset := findMagic(archive, cdOffset)
	if magicOffset < 0 {
		return errors.New("apkbrand: signing block not found")
	}
	blockSize := binary.LittleEndian.Uint64(archive[magicOffset-8:])
	blockStart := magicOffset + 16 - int(blockSize) - 8

	eocd := make([]byte, len(archive)-eocdOffset)
	copy(eocd, archive[eocdOffset:])
	binary.LittleEndian.PutUint32(eocd[16:], uint32(blockStart))

	digest, err := contentDigest(
		archive[:blockStart],
		archive[cdOffset:eocdOffset],
		eocd,
	)
	if err != nil {
		return err
	}
	block := archive[blockStart : magicOffset+16]
	parsed, err := parseSignerBlock(block)
	if err != nil {
		return err
	}
	signedDataHash := sha256.Sum256(parsed.signedData)
	if err := rsa.VerifyPKCS1v15(&signer.PrivateKey.PublicKey, crypto.SHA256, signedDataHash[:], parsed.signature); err != nil {
		return fmt.Errorf("apkbrand: signature verification failed: %w", err)
	}
	if len(parsed.contentDigest) != len(digest) {
		return errors.New("apkbrand: digest length mismatch")
	}
	for i := range digest {
		if digest[i] != parsed.contentDigest[i] {
			return errors.New("apkbrand: content digest mismatch")
		}
	}
	return nil
}

// findMagic locates the APK signing block magic in front of the central directory.
func findMagic(archive []byte, cdOffset uint32) int {
	magic := []byte(apkSigningBlockMagic)
	limit := int(cdOffset) - len(magic)
	if limit < 0 {
		return -1
	}
	for offset := limit; offset >= 0; offset-- {
		if string(archive[offset:offset+len(magic)]) == string(magic) {
			return offset
		}
	}
	return -1
}

// parsedSigner holds the parts of a signer block needed for verification.
type parsedSigner struct {
	// contentDigest is the digest over the protected APK sections.
	contentDigest []byte
	// signedData is the signed data block the signature was created over.
	signedData []byte
	// signature is the RSA signature over the signed data block.
	signature []byte
}

// parseSignerBlock extracts the signed data and the signature of a signing block.
func parseSignerBlock(block []byte) (*parsedSigner, error) {
	offset := 8 // skip the size field
	end := len(block) - len(apkSigningBlockMagic) - 8
	var v2 []byte
	for offset < end {
		pairLength := int(binary.LittleEndian.Uint64(block[offset:]))
		pairID := binary.LittleEndian.Uint32(block[offset+8:])
		value := block[offset+12 : offset+8+pairLength]
		if pairID == apkSignatureSchemeV2BlockID {
			v2 = value
		}
		offset += 8 + pairLength
	}
	if v2 == nil {
		return nil, errors.New("apkbrand: v2 block missing")
	}
	signers, _, err := readLengthPrefixed(v2, 0)
	if err != nil {
		return nil, err
	}
	signer, _, err := readLengthPrefixed(signers, 0)
	if err != nil {
		return nil, err
	}
	signedData, next, err := readLengthPrefixed(signer, 0)
	if err != nil {
		return nil, err
	}
	signatures, _, err := readLengthPrefixed(signer, next)
	if err != nil {
		return nil, err
	}
	// The signature sequence holds one length prefixed element per signature, every
	// element carries the algorithm id followed by the length prefixed signature.
	signatureElement, _, err := readLengthPrefixed(signatures, 0)
	if err != nil {
		return nil, err
	}
	signatureBytes, _, err := readLengthPrefixed(signatureElement, 4)
	if err != nil {
		return nil, err
	}

	digests, _, err := readLengthPrefixed(signedData, 0)
	if err != nil {
		return nil, err
	}
	digestEntry, _, err := readLengthPrefixed(digests, 0)
	if err != nil {
		return nil, err
	}
	digest, _, err := readLengthPrefixed(digestEntry, 4)
	if err != nil {
		return nil, err
	}
	return &parsedSigner{contentDigest: digest, signedData: signedData, signature: signatureBytes}, nil
}

// readLengthPrefixed reads a length prefixed payload.
func readLengthPrefixed(data []byte, offset int) ([]byte, int, error) {
	if offset+4 > len(data) {
		return nil, 0, fmt.Errorf("apkbrand: truncated payload at offset %d of %d", offset, len(data))
	}
	length := int(binary.LittleEndian.Uint32(data[offset:]))
	if offset+4+length > len(data) {
		return nil, 0, fmt.Errorf("apkbrand: truncated payload at offset %d len %d need %d", offset, len(data), offset+4+length)
	}
	return data[offset+4 : offset+4+length], offset + 4 + length, nil
}

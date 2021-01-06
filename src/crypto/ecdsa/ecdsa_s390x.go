// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecdsa

import (
	"crypto/cipher"
	"crypto/elliptic"
	"internal/cpu"
	"math/big"
)

// kdsa invokes the "compute digital signature authentication"
// instruction with the given function code and 4096 byte
// parameter block.
//
// The return value corresponds to the condition code set by the
// instruction. Interrupted invocations are handled by the
// function.
//go:noescape
func kdsa(fc uint64, params *[4096]byte) (errn uint64)

// canUseKDSA checks if KDSA instruction is available, and if it is, it checks
// the name of the curve to see if it matches the curves supported(P-256, P-384, P-521).
// Then, based on the curve name, a function code and a block size will be assigned.
// If KDSA instruction is not available or if the curve is not supported, canUseKDSA
// will set ok to false.
func canUseKDSA(c elliptic.Curve) (functionCode uint64, blockSize int, ok bool) {
	if !cpu.S390X.HasECDSA {
		return 0, 0, false
	}
	switch c.Params().Name {
	case "P-256":
		return 1, 32, true
	case "P-384":
		return 2, 48, true
	case "P-521":
		return 3, 80, true
	}
	return 0, 0, false // A mismatch
}

func hashToBytes(dst, hash []byte, c elliptic.Curve) {
	lenDst := len(dst)
	orderBits := c.Params().N.BitLen()
	fmt.Println("order bits", orderBits)
	if orderBits == lenDst*8 {
		// allocation free path for curves with a length that is a whole number of bytes
		if len(hash) >= lenDst {
			// truncate hash
			copy(dst, hash[:lenDst])
			return
		}
		// pad hash with leading zeros
		p := lenDst - len(hash)
		for i := 0; i < p; i++ {
			dst[i] = 0
		}
		copy(dst[p:], hash)
		return
	}

	// determine the order of the curve in bytes
	orderBytes := (orderBits + 7) / 8

	// fail out early if there are not enough bytes
	if lenDst < orderBytes {
		panic("crypto/ecdsa: dst too small to fit value")
	}

	// pad with zeros
	pad := lenDst - orderBits/8
	for i := 0; i <= pad; i++ {
		dst[i] = 0
	}

	// figure out the excess bytes between the hash size and order of the curve
	excess := lenDst - orderBytes

	// determine the shifts needed
	truncateBits := ((orderBits - 1) % 8) + 1
	shiftBits := (8 - truncateBits) % 8

	// loop over the bytes fill in the dst values
	carry := byte(0)
	for i := 0; i < orderBytes; i++ {
		dst[excess+i] = (hash[i] >> shiftBits)
		dst[excess+i] |= carry
		carry = hash[i] << truncateBits
	}
}

func sign(priv *PrivateKey, csprng *cipher.StreamReader, c elliptic.Curve, hash []byte) (r, s *big.Int, err error) {
	if functionCode, blockSize, ok := canUseKDSA(c); ok {
		for {
			var k *big.Int
			k, err = randFieldElement(c, *csprng)
			if err != nil {
				return nil, nil, err
			}

			// The parameter block looks like the following for sign.
			// 	+---------------------+
			// 	|   Signature(R)      |
			//	+---------------------+
			//	|   Signature(S)      |
			//	+---------------------+
			//	|   Hashed Message    |
			//	+---------------------+
			//	|   Private Key       |
			//	+---------------------+
			//	|   Random Number     |
			//	+---------------------+
			//	|                     |
			//	|        ...          |
			//	|                     |
			//	+---------------------+
			// The common components(signatureR, signatureS, hashedMessage, privateKey and
			// random number) each takes block size of bytes. The block size is different for
			// different curves and is set by canUseKDSA function.
			var params [4096]byte

			// Copy content into the parameter block. In the sign case,
			// we copy hashed message, private key and random number into
			// the parameter block.
			hashToBytes(params[2*blockSize:3*blockSize], hash, c)
			priv.D.FillBytes(params[3*blockSize : 4*blockSize])
			k.FillBytes(params[4*blockSize : 5*blockSize])
			// Convert verify function code into a sign function code by adding 8.
			// We also need to set the 'deterministic' bit in the function code, by
			// adding 128, in order to stop the instruction using its own random number
			// generator in addition to the random number we supply.
			switch kdsa(functionCode+136, &params) {
			case 0: // success
				r = new(big.Int)
				r.SetBytes(params[:blockSize])
				s = new(big.Int)
				s.SetBytes(params[blockSize : 2*blockSize])
				return
			case 1: // error
				return nil, nil, errZeroParam
			case 2: // retry
				continue
			}
			panic("unreachable")
		}
	}
	return signGeneric(priv, csprng, c, hash)
}

func verify(pub *PublicKey, c elliptic.Curve, hash []byte, r, s *big.Int) bool {
	if functionCode, blockSize, ok := canUseKDSA(c); ok {
		// The parameter block looks like the following for verify:
		// 	+---------------------+
		// 	|   Signature(R)      |
		//	+---------------------+
		//	|   Signature(S)      |
		//	+---------------------+
		//	|   Hashed Message    |
		//	+---------------------+
		//	|   Public Key X      |
		//	+---------------------+
		//	|   Public Key Y      |
		//	+---------------------+
		//	|                     |
		//	|        ...          |
		//	|                     |
		//	+---------------------+
		// The common components(signatureR, signatureS, hashed message, public key X,
		// and public key Y) each takes block size of bytes. The block size is different for
		// different curves and is set by canUseKDSA function.
		var params [4096]byte

		// Copy content into the parameter block. In the verify case,
		// we copy signature (r), signature(s), hashed message, public key x component,
		// and public key y component into the parameter block.
		r.FillBytes(params[0*blockSize : 1*blockSize])
		s.FillBytes(params[1*blockSize : 2*blockSize])
		hashToBytes(params[2*blockSize:3*blockSize], hash, c)
		pub.X.FillBytes(params[3*blockSize : 4*blockSize])
		pub.Y.FillBytes(params[4*blockSize : 5*blockSize])
		return kdsa(functionCode, &params) == 0
	}
	return verifyGeneric(pub, c, hash, r, s)
}

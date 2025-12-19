package hash

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/kyverno/kyverno/pkg/cel/utils"
)

type impl struct {
	types.Adapter
}

func (c *impl) sha1_string(arg ref.Val) ref.Val {
	if value, err := utils.ConvertToNative[string](arg); err != nil {
		return types.WrapErr(err)
	} else {
		hasher := sha1.New()
		hasher.Write([]byte(value))

		return c.NativeToValue(hex.EncodeToString(hasher.Sum(nil)))
	}
}

func (c *impl) sha256_string(arg ref.Val) ref.Val {
	if value, err := utils.ConvertToNative[string](arg); err != nil {
		return types.WrapErr(err)
	} else {
		hasher := sha256.New()
		hasher.Write([]byte(value))

		return c.NativeToValue(hex.EncodeToString(hasher.Sum(nil)))
	}
}

func (c *impl) md5_string(arg ref.Val) ref.Val {
	if value, err := utils.ConvertToNative[string](arg); err != nil {
		return types.WrapErr(err)
	} else {
		hasher := md5.New()
		hasher.Write([]byte(value))

		return c.NativeToValue(hex.EncodeToString(hasher.Sum(nil)))
	}
}

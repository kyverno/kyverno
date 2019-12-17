package builtins

func _() {
	//@complete("", append, bool, byte, cap, close, complex, complex128, complex64, copy, delete, error, _false, float32, float64, imag, int, int16, int32, int64, int8, len, make, new, panic, print, println, real, recover, rune, string, _true, uint, uint16, uint32, uint64, uint8, uintptr, _nil)
}

/* Create markers for builtin types. Only for use by this test.
/* append(slice []Type, elems ...Type) []Type */ //@item(append, "append", "func(slice []Type, elems ...Type) []Type", "func")
/* bool */ //@item(bool, "bool", "", "type")
/* byte */ //@item(byte, "byte", "", "type")
/* cap(v Type) int */ //@item(cap, "cap", "func(v Type) int", "func")
/* close(c chan<- Type) */ //@item(close, "close", "func(c chan<- Type)", "func")
/* complex(r float64, i float64) */ //@item(complex, "complex", "func(r float64, i float64) complex128", "func")
/* complex128 */ //@item(complex128, "complex128", "", "type")
/* complex64 */ //@item(complex64, "complex64", "", "type")
/* copy(dst []Type, src []Type) int */ //@item(copy, "copy", "func(dst []Type, src []Type) int", "func")
/* delete(m map[Type]Type1, key Type) */ //@item(delete, "delete", "func(m map[Type]Type1, key Type)", "func")
/* error */ //@item(error, "error", "", "interface")
/* false */ //@item(_false, "false", "", "const")
/* float32 */ //@item(float32, "float32", "", "type")
/* float64 */ //@item(float64, "float64", "", "type")
/* imag(c complex128) float64 */ //@item(imag, "imag", "func(c complex128) float64", "func")
/* int */ //@item(int, "int", "", "type")
/* int16 */ //@item(int16, "int16", "", "type")
/* int32 */ //@item(int32, "int32", "", "type")
/* int64 */ //@item(int64, "int64", "", "type")
/* int8 */ //@item(int8, "int8", "", "type")
/* iota */ //@item(iota, "iota", "", "const")
/* len(v Type) int */ //@item(len, "len", "func(v Type) int", "func")
/* make(t Type, size ...int) Type */ //@item(make, "make", "func(t Type, size ...int) Type", "func")
/* new(Type) *Type */ //@item(new, "new", "func(Type) *Type", "func")
/* nil */ //@item(_nil, "nil", "", "var")
/* panic(v interface{}) */ //@item(panic, "panic", "func(v interface{})", "func")
/* print(args ...Type) */ //@item(print, "print", "func(args ...Type)", "func")
/* println(args ...Type) */ //@item(println, "println", "func(args ...Type)", "func")
/* real(c complex128) float64 */ //@item(real, "real", "func(c complex128) float64", "func")
/* recover() interface{} */ //@item(recover, "recover", "func() interface{}", "func")
/* rune */ //@item(rune, "rune", "", "type")
/* string */ //@item(string, "string", "", "type")
/* true */ //@item(_true, "true", "", "const")
/* uint */ //@item(uint, "uint", "", "type")
/* uint16 */ //@item(uint16, "uint16", "", "type")
/* uint32 */ //@item(uint32, "uint32", "", "type")
/* uint64 */ //@item(uint64, "uint64", "", "type")
/* uint8 */ //@item(uint8, "uint8", "", "type")
/* uintptr */ //@item(uintptr, "uintptr", "", "type")

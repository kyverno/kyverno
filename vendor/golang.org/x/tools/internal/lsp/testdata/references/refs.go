package refs

type i int //@mark(typeI, "i"),refs("i", typeI, argI, returnI)

func _(_ i) []bool { //@mark(argI, "i")
	return nil
}

func _(_ []byte) i { //@mark(returnI, "i")
	return 0
}

var q string //@mark(declQ, "q"),refs("q", declQ, assignQ, bobQ)

var Q string //@mark(declExpQ, "Q"), refs("Q", declExpQ, assignExpQ, bobExpQ)

func _() {
	q = "hello" //@mark(assignQ, "q")
	bob := func(_ string) {}
	bob(q) //@mark(bobQ, "q")
}

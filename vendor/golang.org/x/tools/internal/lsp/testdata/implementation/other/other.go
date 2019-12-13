package other

type ImpP struct{} //@mark(OtherImpP, "ImpP")

func (*ImpP) Laugh() { //@mark(OtherLaughP, "Laugh")
}

type ImpS struct{} //@mark(OtherImpS, "ImpS")

func (ImpS) Laugh() { //@mark(OtherLaughS, "Laugh")
}

type ImpI interface { //@mark(OtherImpI, "ImpI")
	Laugh() //@mark(OtherLaughI, "Laugh")
}

type Foo struct {
}

func (Foo) U() { //@mark(ImpU, "U")
}
<<<<<<< HEAD

type CryType int

const Sob CryType = 1

type Cryer interface {
	Cry(CryType) //@implementations("Cry", CryImpl)
}
=======
>>>>>>> 524_bug

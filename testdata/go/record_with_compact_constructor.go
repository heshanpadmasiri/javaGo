package converted

type Rational struct {
	Num   int
	Denom int
}

func newRationalFromNumDenom(num int, denom int) Rational {
	this := Rational{}
	if denom == 0 {
		panic(("Denominator cannot be zero"))
	}
	if (num < 0) && (denom < 0) {
		num = (-num)
		denom = (-denom)
	}
	this.Num = num
	this.Denom = denom
	return this
}

func NewRational() Rational {
	this := Rational{}
	return this
}

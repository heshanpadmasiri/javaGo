package converted

type calculator struct {
}

func newCalculator() calculator {
	this := calculator{}
	return this
}

func (this *calculator) Add(a int, b int) int {
	return (a + b)
}

func (this *calculator) AddWithFloat64Float64(a float64, b float64) float64 {
	return (a + b)
}

func (this *calculator) AddWithStringString(a string, b string) string {
	return (a + b)
}

func (this *calculator) Test() {
	// FIXME: more than one possible method for add with 2 arguments

	x := this.Add(1, 2)
	// FIXME: more than one possible method for add with 2 arguments

	y := this.Add(1.0, 2.0)
	// FIXME: more than one possible method for add with 2 arguments

	z := this.Add("Hello", "World")
}

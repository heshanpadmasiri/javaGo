package converted

type calculator struct {
}

func newCalculator() calculator {
	this := calculator{}
	return this
}

func (this *calculator) Add(a int, b int) int {
	// migrated from overloaded_methods_different_types.java:2:5
	return (a + b)
}

func (this *calculator) AddWithFloat64Float64(a float64, b float64) float64 {
	// migrated from overloaded_methods_different_types.java:6:5
	return (a + b)
}

func (this *calculator) AddWithStringString(a string, b string) string {
	// migrated from overloaded_methods_different_types.java:10:5
	return (a + b)
}

func (this *calculator) Test() {
	// migrated from overloaded_methods_different_types.java:14:5
	// FIXME: more than one possible method for add with 2 arguments

	x := this.Add(1, 2)
	// FIXME: more than one possible method for add with 2 arguments

	y := this.Add(1.0, 2.0)
	// FIXME: more than one possible method for add with 2 arguments

	z := this.Add("Hello", "World")
}

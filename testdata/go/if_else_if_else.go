package converted

type test struct {
}

func (this *test) test() {
	if x > 0 {
		this.positive()
	} else if x < 0 {
		this.negative()
	} else {
		this.zero()
	}
}

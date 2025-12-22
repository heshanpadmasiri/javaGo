package converted

type test struct {
}

func (this *test) test() {
	if x {
		if y {
			this.a()
		} else {
			this.b()
		}
	} else {
		this.c()
	}
}

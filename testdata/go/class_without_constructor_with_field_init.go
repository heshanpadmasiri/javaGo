package converted

type WithDefaults struct {
	value int
	name  string
}

func NewWithDefaults() WithDefaults {
	this := WithDefaults{}
	this.value = 42
	this.name = "default"
	// Default field initializations

	return this
}

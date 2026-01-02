package converted

type WithDefaults struct {
	value int
	name  string
}

func NewWithDefaults() WithDefaults {
	this := WithDefaults{}
	this.name = "default"
	this.value = 42
	// Default field initializations

	return this
}

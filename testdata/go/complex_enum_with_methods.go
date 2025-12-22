package converted

type Status struct {
	value string
}

var Status_ACTIVE = Status{value: "active"}
var Status_INACTIVE = Status{value: "inactive"}

func FromString(s string) Status {
	if "active" == s {
		return Status_ACTIVE
	}
	return Status_INACTIVE
}

func (this *Status) GetValue() string {
	return this.value
}

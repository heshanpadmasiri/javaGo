public enum Status {
    ACTIVE("active"),
    INACTIVE("inactive");

    private String value;

    Status(String value) {
        this.value = value;
    }

    public String getValue() {
        return this.value;
    }

    public static Status fromString(String s) {
        if ("active".equals(s)) {
            return ACTIVE;
        }
        return INACTIVE;
    }
}


class Test {
    // Single constructor with matching params - should work cleanly
    public static final Test INSTANCE = new Test(42, "example");

    // Test with multiple overloaded constructors (ambiguous by count)
    public static final Test AMBIGUOUS = new Test(0, 0, 0);

    private int value;
    private String name;

    public Test(int value, String name) {
        this.value = value;
        this.name = name;
    }

    // Create ambiguity: multiple constructors with 3 params
    public Test(int a, int b, int c) {
        this.value = a;
    }

    public Test(int x, String y, int z) {
        this.value = x;
        this.name = y;
    }
}

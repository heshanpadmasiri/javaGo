class Test {
    int calculate() {
        int result;
        try {
            result = compute();
        } catch (RuntimeException e) {
            result = defaultValue();
        }
        return result;
    }
}
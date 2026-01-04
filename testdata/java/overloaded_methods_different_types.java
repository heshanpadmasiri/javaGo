class Calculator {
    public int add(int a, int b) {
        return a + b;
    }
    
    public double add(double a, double b) {
        return a + b;
    }
    
    public String add(String a, String b) {
        return a + b;
    }
    
    public void test() {
        int x = this.add(1, 2);
        double y = this.add(1.0, 2.0);
        String z = this.add("Hello", "World");
    }
}

class Container {
    private Object value;
    
    public Container(String s) {
        this.value = s;
    }
    
    public Container(Integer i) {
        this.value = i;
    }
    
    public static void test() {
        Container c = new Container("test");
    }
}

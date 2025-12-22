public record Point(int x, int y) {
    public int distance() {
        return x * x + y * y;
    }
}


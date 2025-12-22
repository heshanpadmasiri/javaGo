public interface Drawable {
    void draw();
}

public class Circle implements Drawable {
    private int radius;

    public Circle(int radius) {
        this.radius = radius;
    }

    public void draw() {
        System.out.println("Drawing circle with radius " + radius);
    }
}


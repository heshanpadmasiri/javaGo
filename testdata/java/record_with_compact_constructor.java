public record Rational(int num, int denom) {
    Rational {
        if (denom == 0) {
            throw new IllegalArgumentException("Denominator cannot be zero");
        }
        if (num < 0 && denom < 0) {
            num = -num;
            denom = -denom;
        }
    }
}


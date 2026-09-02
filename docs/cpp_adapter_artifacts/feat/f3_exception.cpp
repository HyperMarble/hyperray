// exceptions: throw / catch
extern "C" int nondet_int();
int risky(int x) { if (x < 0) throw x; return x * 2; }
int main() {
    int x = nondet_int();
    __ESBMC_assume(x >= -5 && x <= 5);
    int r = 0;
    try { r = risky(x); } catch (int e) { r = -e; }
    __ESBMC_assert(r >= 0, "r nonnegative");
    return 0;
}

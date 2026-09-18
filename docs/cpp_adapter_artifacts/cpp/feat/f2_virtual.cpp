// virtual dispatch through a base pointer
extern "C" int nondet_int();
struct Base { virtual int f() { return 1; } virtual ~Base() {} };
struct D1 : Base { int f() override { return 2; } };
struct D2 : Base { int f() override { return 0; } };
int main() {
    int c = nondet_int();
    Base *p = (c > 0) ? (Base*)new D1() : (Base*)new D2();
    int v = p->f();
    __ESBMC_assert(v != 0, "f never returns 0");   // FALSE: D2 returns 0
    return 0;
}

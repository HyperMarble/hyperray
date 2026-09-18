// b3: null pointer dereference
#include <cstddef>
extern "C" int nondet_int();
int *maybe(int c) { return c > 0 ? new int(5) : nullptr; }
int main() {
    int c = nondet_int();
    int *p = maybe(c);
    return *p;                        // null deref when c <= 0
}

// b1: unsigned subtraction underflow — the C++ analogue of builder.rs:322
#include <cstdint>
#include <cstddef>
#include <vector>
extern "C" size_t nondet_size_t();
extern "C" int64_t nondet_int64_t();
int64_t seek_current(size_t prefix_len, size_t position, int64_t offset) {
    size_t unread = prefix_len - position;      // underflow if position > prefix_len
    return offset - (int64_t)unread;            // signed overflow for extreme offset
}
int main() {
    size_t prefix_len = nondet_size_t();
    size_t position   = nondet_size_t();
    int64_t offset    = nondet_int64_t();
    __ESBMC_assume(prefix_len <= 4);
    __ESBMC_assume(position <= 4);
    seek_current(prefix_len, position, offset);
    return 0;
}

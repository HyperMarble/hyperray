// b5: CORRECT version of b1 — must verify SUCCESSFUL (false-positive check)
#include <cstdint>
#include <cstddef>
extern "C" size_t nondet_size_t();
extern "C" int64_t nondet_int64_t();
int64_t seek_current_fixed(size_t prefix_len, size_t position, int64_t offset) {
    size_t unread = (position >= prefix_len) ? 0 : prefix_len - position;
    if (unread > (size_t)INT64_MAX) return offset;
    int64_t u = (int64_t)unread;
    if (offset < INT64_MIN + u) return INT64_MIN;
    return offset - u;
}
int main() {
    size_t prefix_len = nondet_size_t();
    size_t position   = nondet_size_t();
    int64_t offset    = nondet_int64_t();
    __ESBMC_assume(prefix_len <= 4);
    __ESBMC_assume(position <= 4);
    seek_current_fixed(prefix_len, position, offset);
    return 0;
}

// END-TO-END: invariant below is LoopInvGen's output for loop2_detect_scan.sl,
//   (and (or (not (>= i n)) (= seen (* 2 n))) (= (* 2 i) seen))
// translated SyGuS -> C++ mechanically. No human invention.
extern "C" unsigned nondet_uint();
int main() {
    unsigned n = nondet_uint();
    __ESBMC_assume(n <= 65536);          // cap cited from MAX_DETECTION_PREFIX_LEN
    unsigned i = 0, seen = 0;
    __ESBMC_loop_invariant(((!(i >= n)) || (seen == 2*n)) && (2*i == seen));
    while (i < n) { i++; seen += 2; }
    __ESBMC_assert(seen == 2*n, "detect_scan returns 2n");
    return 0;
}

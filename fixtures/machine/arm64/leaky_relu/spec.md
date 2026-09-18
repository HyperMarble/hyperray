# Leaky ReLU specification

Function: `arm64_leaky_relu(values: &mut [i32; 8]) -> u64`

Values use signed Q16.16 fixed-point encoding. The negative slope is 1/8.

For each lane `x`, the stored result is `x` when `x >= 0`. The stored result
is the arithmetic fixed-point shift `floor(x / 8)` when `x < 0`.

The return value equals the number of lanes that were negative before the call.

The function writes only the 32-byte input buffer and its declared stack area.

// replay_reader.cpp — C++ analogue of the noodles-296 ReplayReader patch.
// Deliberately contains one real defect (unsigned/signed subtraction underflow)
// in seek_current(), mirroring builder.rs:322.

#include <cstdint>
#include <cstddef>
#include <vector>
#include <string>

namespace hyperray {

static const size_t MAX_DETECTION_PREFIX_LEN = 65536;
static const size_t SCRATCH_STEP = 8192;

enum class Format { Unknown, Bam, Cram, Vcf };

enum class SeekWhence { Start, End, Current };

// --- stage-2 loop #1: bounded buffer fill ------------------------------------
// fills up to cap, SCRATCH_STEP at a time. Guard is data dependent on `got`.
size_t read_at_least(size_t want, size_t cap) {
    size_t len = 0;
    while (len < want && len < cap) {
        size_t got = SCRATCH_STEP;
        if (len + got > cap) {
            got = cap - len;
        }
        len = len + got;
    }
    return len;
}

// --- stage-2 loop #2: two coupled counters -----------------------------------
size_t detect_scan(size_t n) {
    size_t i = 0;
    size_t seen = 0;
    while (i < n) {
        i = i + 1;
        seen = seen + 2;
    }
    return seen;
}

size_t scratch_len(size_t dst_len) {
    if (dst_len >= MAX_DETECTION_PREFIX_LEN) {
        return 0;
    }
    size_t room = MAX_DETECTION_PREFIX_LEN - dst_len;
    return room < SCRATCH_STEP ? room : SCRATCH_STEP;
}

class ReplayReader {
public:
    std::vector<uint8_t> prefix;
    size_t position;

    ReplayReader(std::vector<uint8_t> p, size_t pos) : prefix(p), position(pos) {}

    // DEFECT: prefix.size() - position underflows when position > size();
    // then the int64_t subtraction can overflow for extreme offsets.
    int64_t seek_current(int64_t offset) {
        size_t unread_prefix_len = prefix.size() - position;
        return offset - (int64_t)unread_prefix_len;
    }

    uint8_t at(size_t idx) {
        return prefix[idx];        // no bounds check: OOB read
    }
};

Format detect_format(const std::vector<uint8_t>& buf) {
    if (buf.size() < 4) {
        return Format::Unknown;
    }
    if (buf[0] == 'B' && buf[1] == 'A' && buf[2] == 'M') {
        return Format::Bam;
    }
    if (buf[0] == 'C' && buf[1] == 'R' && buf[2] == 'A' && buf[3] == 'M') {
        return Format::Cram;
    }
    return Format::Unknown;
}

template <typename T>
T clamp_add(T a, T b, T hi) {
    T s = a + b;
    if (s > hi) return hi;
    return s;
}

} // namespace hyperray

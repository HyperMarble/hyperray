// This source supplies a bounded no_std ARM64 packet processor fixture.
// It must not allocate, call an operating system, or use inline assembly.
#![no_std]

const MAX_PACKET_BYTES: usize = 32;
const HEADER_BYTES: usize = 4;
const MAX_PAYLOAD_BYTES: usize = MAX_PACKET_BYTES - HEADER_BYTES;
const VERSION: u8 = 2;
const RESERVED_FLAGS: u8 = 0xe0;

#[repr(u8)]
#[derive(Clone, Copy)]
enum PacketError {
    TooShort = 1,
    ReservedFlags = 2,
    Version = 3,
    Kind = 4,
    PayloadLength = 5,
    Truncated = 6,
    Stream = 7,
    Checksum = 8,
}

impl PacketError {
    #[inline(never)]
    fn code(self) -> u64 {
        0x8000_0000_0000_0000 | self as u64
    }
}

#[derive(Clone, Copy)]
struct Header {
    version: u8,
    kind: u8,
    stream: u8,
    payload_length: usize,
    expected_checksum: u8,
}

#[inline(never)]
fn parse_header(input: &[u8; MAX_PACKET_BYTES], input_length: usize) -> Result<Header, PacketError> {
    if input_length < HEADER_BYTES {
        return Err(PacketError::TooShort);
    }
    let flags = input[0];
    if flags & RESERVED_FLAGS != 0 {
        return Err(PacketError::ReservedFlags);
    }
    let version = flags & 0x07;
    let kind = (flags >> 3) & 0x03;
    if version != VERSION {
        return Err(PacketError::Version);
    }
    if kind == 0 {
        return Err(PacketError::Kind);
    }
    let payload_length = input[1] as usize;
    if payload_length > MAX_PAYLOAD_BYTES {
        return Err(PacketError::PayloadLength);
    }
    if input_length < HEADER_BYTES + payload_length {
        return Err(PacketError::Truncated);
    }
    let stream = input[2];
    if stream == 0 {
        return Err(PacketError::Stream);
    }
    Ok(Header {
        version,
        kind,
        stream,
        payload_length,
        expected_checksum: input[3],
    })
}

#[inline(never)]
fn checksum_payload(input: &[u8; MAX_PACKET_BYTES], payload_length: usize) -> Result<u8, PacketError> {
    let mut checksum = 0x5a_u8;
    let mut index = 0;
    while index < payload_length {
        let byte = match input.get(HEADER_BYTES + index) {
            Some(byte) => *byte,
            None => return Err(PacketError::PayloadLength),
        };
        checksum = checksum.rotate_left(1) ^ byte;
        checksum = checksum.wrapping_add((index as u8).wrapping_mul(3));
        index += 1;
    }
    Ok(checksum)
}

#[inline(never)]
fn success_code(header: Header, checksum: u8) -> u64 {
    u64::from(header.version) << 56
        | u64::from(header.kind) << 52
        | u64::from(header.stream) << 32
        | u64::from(checksum) << 24
        | header.payload_length as u64
}

#[no_mangle]
pub extern "C" fn arm64_packet_process(
    input: &[u8; MAX_PACKET_BYTES],
    input_length: usize,
) -> u64 {
    let header = match parse_header(input, input_length) {
        Ok(header) => header,
        Err(error) => return error.code(),
    };
    let checksum = match checksum_payload(input, header.payload_length) {
        Ok(checksum) => checksum,
        Err(error) => return error.code(),
    };
    if checksum != header.expected_checksum {
        return PacketError::Checksum.code();
    }
    success_code(header, checksum)
}

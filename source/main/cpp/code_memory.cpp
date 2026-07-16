#include "ccova/code_memory.h"
#include "ccova/byte_order.h"

namespace ncore
{
    static inline void assert_code_range(const code_memory_t* memory, u32 offset, u32 size)
    {
        ASSERT(memory != nullptr);
        ASSERT(memory->m_code != nullptr || memory->m_size == 0);
        ASSERT(offset <= memory->m_size);
        ASSERT(size <= memory->m_size - offset);
    }

    instruction_t read_instruction(const code_memory_t* memory, u32* offset)
    {
        ASSERT(offset != nullptr);
        assert_code_range(memory, *offset, 2);
        const instruction_t instruction = read_le_u16(memory->m_code + *offset);
        *offset += 2;
        return instruction;
    }

    u64 read_immediate(const code_memory_t* memory, u32* offset, evaluekind_t kind)
    {
        ASSERT(offset != nullptr);
        const u32 size = value_kind_size(kind);
        ASSERT(size != 0);
        assert_code_range(memory, *offset, size);
        const byte* data = memory->m_code + *offset;
        *offset += size;
        switch (size)
        {
            case 1: return data[0];
            case 2: return read_le_u16(data);
            case 4: return read_le_u32(data);
            case 8: return read_le_u64(data);
            default: ASSERT(false); return 0;
        }
    }

    u32 read_u32(const code_memory_t* memory, u32* offset)
    {
        ASSERT(offset != nullptr);
        assert_code_range(memory, *offset, 4);
        const u32 value = read_le_u32(memory->m_code + *offset);
        *offset += 4;
        return value;
    }
} // namespace ncore
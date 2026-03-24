/*
 * dump_typeinfo.c - Dumps XTypes TypeInformation bytes from Cyclone DDS
 * generated type descriptors.
 *
 * The idlc compiler generates a dds_topic_descriptor_t for each top-level
 * struct, which contains a type_information field with the serialized
 * TypeInformation bytes (including the CDR encapsulation header).
 *
 * This program prints each type's TypeInformation as:
 *   TYPE_NAME <hex bytes>
 *
 * Usage: compile with the idlc-generated header/source, link against CycloneDDS.
 */

#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include "test_types.h"

/* Print TypeInformation bytes for a single topic descriptor */
static void dump_descriptor(const char *name, const dds_topic_descriptor_t *desc)
{
    printf("%s ", name);

    if (desc->type_information.data == NULL || desc->type_information.sz == 0) {
        printf("(no type information)\n");
        return;
    }

    const unsigned char *data = (const unsigned char *)desc->type_information.data;
    uint32_t sz = desc->type_information.sz;

    /* Print size first */
    printf("size=%u bytes=", sz);

    /* Print hex bytes */
    for (uint32_t i = 0; i < sz; i++) {
        printf("%02x", data[i]);
    }
    printf("\n");
}

/* Also dump the TypeObject (type_mapping) for debugging */
static void dump_typemap(const char *name, const dds_topic_descriptor_t *desc)
{
    printf("%s_TYPEMAP ", name);

    if (desc->type_mapping.data == NULL || desc->type_mapping.sz == 0) {
        printf("(no type mapping)\n");
        return;
    }

    const unsigned char *data = (const unsigned char *)desc->type_mapping.data;
    uint32_t sz = desc->type_mapping.sz;

    printf("size=%u bytes=", sz);

    for (uint32_t i = 0; i < sz; i++) {
        printf("%02x", data[i]);
    }
    printf("\n");
}

int main(void)
{
    printf("# Cyclone DDS TypeInformation dump\n");
    printf("# Format: TYPE_NAME size=N bytes=<hex>\n");
    printf("#\n");

    /* Dump TypeInformation for each test type */
    dump_descriptor("SimpleType",       &test_SimpleType_desc);
    dump_descriptor("SensorData",       &test_SensorData_desc);
    dump_descriptor("ArrayType",        &test_ArrayType_desc);
    dump_descriptor("SequenceType",     &test_SequenceType_desc);
    dump_descriptor("EnumStruct",       &test_EnumStruct_desc);
    dump_descriptor("OuterType",        &test_OuterType_desc);
    dump_descriptor("AppendableType",   &test_AppendableType_desc);
    dump_descriptor("MutableType",      &test_MutableType_desc);
    dump_descriptor("BoundedStringType",&test_BoundedStringType_desc);

    printf("\n# TypeMapping (TypeObject) data for reference\n");

    dump_typemap("SimpleType",       &test_SimpleType_desc);
    dump_typemap("SensorData",       &test_SensorData_desc);
    dump_typemap("ArrayType",        &test_ArrayType_desc);
    dump_typemap("SequenceType",     &test_SequenceType_desc);
    dump_typemap("EnumStruct",       &test_EnumStruct_desc);
    dump_typemap("OuterType",        &test_OuterType_desc);
    dump_typemap("AppendableType",   &test_AppendableType_desc);
    dump_typemap("MutableType",      &test_MutableType_desc);
    dump_typemap("BoundedStringType",&test_BoundedStringType_desc);

    return 0;
}

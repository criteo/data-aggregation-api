set -e

VERSION=a8df3673d88bf422b51a70093878d3c659786d5f

rm -rf internal/model/openconfig/*
rm -rf internal/model/ietf/*
rm .build -rf
mkdir .build && cd .build

git clone https://github.com/openconfig/ygot.git --depth 1 && cd ygot
git clone https://github.com/criteo-forks/openconfig-public.git public --depth 1
git clone https://github.com/YangModels/yang.git --depth 1

git -C public fetch --tags
git -C public checkout $VERSION
mkdir openconfig

go run generator/generator.go -path=public,deps -output_file=openconfig/oc.go \
  -generate_path_structs -path_structs_output_file=openconfig/oc_path.go \
  -package_name=openconfig -generate_fakeroot -fakeroot_name=device -compress_paths=true \
  -shorten_enum_leaf_names \
  -trim_enum_openconfig_prefix \
  -typedef_enum_with_defmod \
  -enum_suffix_for_simple_union_enums \
  -exclude_modules=ietf-interfaces \
  -generate_rename \
  -generate_append \
  -generate_getters \
  -generate_leaf_getters \
  -generate_simple_unions \
  -annotations \
  -list_builder_key_threshold=3 \
  public/release/models/network-instance/openconfig-network-instance.yang \
  public/release/models/bgp/openconfig-bgp.yang \
  public/release/models/policy/openconfig-routing-policy.yang \
  public/release/models/bgp/openconfig-bgp-policy.yang

go run generator/generator.go -path=yang,deps -output_file=ietf \
  -package_name=ietf -generate_fakeroot -fakeroot_name=device \
  -shorten_enum_leaf_names \
  -trim_enum_openconfig_prefix \
  -typedef_enum_with_defmod \
  -enum_suffix_for_simple_union_enums \
  -exclude_modules=ietf-interfaces \
  -generate_rename \
  -generate_append \
  -generate_getters \
  -generate_leaf_getters \
  -generate_simple_unions \
  -annotations \
  -list_builder_key_threshold=3 \
  yang/standard/ietf/RFC/ietf-system.yang \
  yang/standard/ietf/RFC/ietf-snmp.yang \
  yang/standard/ietf/RFC/ietf-snmp-community.yang \



mv openconfig/* ../../internal/model/openconfig/
mv ietf ../../internal/model/ietf/ietf.go

cd ../../
rm .build -rf

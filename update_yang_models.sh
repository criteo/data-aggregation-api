set -e

# Upstream openconfig version
VERSION=f34434149a47aa8ff82ffd32add3aacb7c880af2

# ygot generator version. Must match the github.com/openconfig/ygot version in
# go.mod: the generated code is compiled against that library.
YGOT_VERSION=v0.35.0

GENERATOR="go run github.com/openconfig/ygot/generator@$YGOT_VERSION"

# Flags shared by both generations. They shape the generated Go API, so
# changing any of them renames types or methods used across internal/.
COMMON_FLAGS="-generate_fakeroot -fakeroot_name=device \
  -shorten_enum_leaf_names \
  -trim_enum_openconfig_prefix \
  -typedef_enum_with_defmod \
  -enum_suffix_for_simple_union_enums \
  -exclude_modules=ietf-interfaces \
  -generate_simple_unions \
  -list_builder_key_threshold=3"

rm -rf internal/model/openconfig/*
rm -rf internal/model/ietf/*
rm .build -rf
mkdir .build && cd .build

# openconfig-public: release/models holds the OpenConfig modules, third_party
# the IETF modules they import (ietf-yang-types, ietf-inet-types, ...).
git clone --depth 1 --filter=blob:none --sparse https://github.com/openconfig/public.git public
git -C public sparse-checkout set release/models third_party
git -C public checkout $VERSION

# YangModels/yang: only the IETF RFC modules are needed, out of ~174k files.
git clone --depth 1 --filter=blob:none --sparse https://github.com/YangModels/yang.git
git -C yang sparse-checkout set standard/ietf/RFC

# AFK augmentations
cp ../yang/criteo/*.yang public/release/models/

mkdir openconfig

$GENERATOR -path=public -output_file=openconfig/oc.go \
  -package_name=openconfig -compress_paths=true \
  $COMMON_FLAGS \
  public/release/models/network-instance/openconfig-network-instance.yang \
  public/release/models/policy/openconfig-routing-policy.yang \
  public/release/models/bgp/openconfig-bgp-policy.yang \
  public/release/models/criteo-bgp-ext.yang \
  public/release/models/criteo-oc-deviations.yang

$GENERATOR -path=yang -output_file=ietf \
  -package_name=ietf \
  $COMMON_FLAGS \
  yang/standard/ietf/RFC/ietf-system.yang \
  yang/standard/ietf/RFC/ietf-snmp.yang \
  yang/standard/ietf/RFC/ietf-snmp-community.yang

mv openconfig/* ../internal/model/openconfig/
mv ietf ../internal/model/ietf/ietf.go

cd ..
rm .build -rf

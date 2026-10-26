#!/bin/bash -eu

# This script is only meant to be run by OSS-Fuzz.
# OSS-Fuzz uses this script to compile kyvernos fuzzers

compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/utils/api FuzzJmespath FuzzJmespath
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v2beta1 FuzzV2beta1PolicyValidate FuzzV2beta1PolicyValidate
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v2beta1 FuzzV2beta1ImageVerification FuzzV2beta1ImageVerification
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v2beta1 FuzzV2beta1MatchResources FuzzV2beta1MatchResources
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v1 FuzzV1PolicyValidate FuzzV2beta1PolicyValidate
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v1 FuzzV1ImageVerification FuzzV2beta1ImageVerification
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v1 FuzzV1MatchResources FuzzV2beta1MatchResources
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine/anchor FuzzAnchorParseTest FuzzAnchorParseTest
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine/api FuzzEngineResponse FuzzEngineResponse

cp $SRC/kyverno/test/fuzz/options/* $OUT/

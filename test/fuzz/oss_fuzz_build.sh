#!/bin/bash -eu

# This script is only meant to be run by OSS-Fuzz.
# OSS-Fuzz uses this script to compile kyvernos fuzzers

# Needed by OSS-Fuzz:
printf "package engine\nimport _ \"github.com/AdamKorcz/go-118-fuzz-build/testing\"\n" > $SRC/kyverno/pkg/engine/registerfuzzdep.go
go mod tidy

compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine/variables FuzzEvaluate FuzzEvaluate
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v2beta1 FuzzV2beta1PolicyValidate FuzzV2beta1PolicyValidate
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v2beta1 FuzzV2beta1ImageVerification FuzzV2beta1ImageVerification
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v2beta1 FuzzV2beta1MatchResources FuzzV2beta1MatchResources
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v2beta1 FuzzV2beta1ClusterPolicy FuzzV2beta1ClusterPolicy
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v1 FuzzV1PolicyValidate FuzzV2beta1PolicyValidate
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v1 FuzzV1ImageVerification FuzzV2beta1ImageVerification
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v1 FuzzV1MatchResources FuzzV2beta1MatchResources
compile_native_go_fuzzer github.com/kyverno/kyverno/api/kyverno/v1 FuzzV1ClusterPolicy FuzzV2beta1ClusterPolicy
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine FuzzVerifyImageAndPatchTest FuzzVerifyImageAndPatchTest
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine FuzzEngineValidateTest FuzzEngineValidateTest
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine FuzzMutateTest FuzzMutateTest
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/validation/policy FuzzValidatePolicy FuzzValidatePolicy
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine/anchor FuzzAnchorParseTest FuzzAnchorParseTest
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine/api FuzzEngineResponse FuzzEngineResponse

cp $SRC/kyverno/test/fuzz/dictionaries/fuzz.dict $OUT/FuzzEngineValidateTest.dict
cp $SRC/kyverno/test/fuzz/dictionaries/fuzz.dict $OUT/FuzzMutateTest.dict
cp $SRC/kyverno/test/fuzz/dictionaries/fuzz.dict $OUT/FuzzVerifyImageAndPatchTest.dict

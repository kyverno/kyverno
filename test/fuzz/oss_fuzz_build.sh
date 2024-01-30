#!/bin/bash -eu

# This script is only meant to be run by OSS-Fuzz.
# OSS-Fuzz uses this script to compile kyvernos fuzzers

go get github.com/kyverno/go-jmespath@bf1569660fd8c66aa7869fce7e56606dda285433
go mod edit -replace github.com/AdaLogics/go-fuzz-headers=github.com/AdamKorcz/go-fuzz-headers-1@8b5d3ce2d11de86b1af0054d9187b6261d0d69d3
# Needed by OSS-Fuzz:
printf "package engine\nimport _ \"github.com/AdamKorcz/go-118-fuzz-build/testing\"\n" > $SRC/kyverno/pkg/engine/registerfuzzdep.go
go mod tidy

compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/utils/api FuzzJmespath FuzzJmespath
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
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine FuzzPodBypass FuzzPodBypass
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine FuzzMutateTest FuzzMutateTest
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/validation/policy FuzzValidatePolicy FuzzValidatePolicy
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine/anchor FuzzAnchorParseTest FuzzAnchorParseTest
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine/api FuzzEngineResponse FuzzEngineResponse
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/engine/context FuzzHasChanged FuzzHasChanged
compile_native_go_fuzzer github.com/kyverno/kyverno/pkg/pss FuzzBaselinePS FuzzBaselinePS

cp $SRC/kyverno/test/fuzz/dictionaries/fuzz.dict $OUT/FuzzEngineValidateTest.dict
cp $SRC/kyverno/test/fuzz/dictionaries/fuzz.dict $OUT/FuzzMutateTest.dict
cp $SRC/kyverno/test/fuzz/dictionaries/fuzz.dict $OUT/FuzzVerifyImageAndPatchTest.dict

zip $OUT/FuzzBaselinePS_seed_corpus.zip $SRC/kyverno/test/fuzz/seeds/FuzzBaselinePS_seed*
zip $OUT/FuzzPodBypass_seed_corpus.zip $SRC/kyverno/test/fuzz/seeds/FuzzPodBypass_seed*

cp $SRC/kyverno/test/fuzz/options/* $OUT/

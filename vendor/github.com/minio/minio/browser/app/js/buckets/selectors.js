/*
 * MinIO Cloud Storage (C) 2018 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { createSelector } from "reselect"

const bucketsSelector = state => state.buckets.list
const bucketsFilterSelector = state => state.buckets.filter

export const getFilteredBuckets = createSelector(
  bucketsSelector,
  bucketsFilterSelector,
  (buckets, filter) => buckets.filter(bucket => bucket.indexOf(filter) > -1)
)

export const getCurrentBucket = state => state.buckets.currentBucket

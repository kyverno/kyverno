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

import React from "react"
import { shallow } from "enzyme"
import { Bucket } from "../Bucket"

describe("Bucket", () => {
  it("should render without crashing", () => {
    shallow(<Bucket />)
  })

  it("should call selectBucket when clicked", () => {
    const selectBucket = jest.fn()
    const wrapper = shallow(
      <Bucket bucket={"test"} selectBucket={selectBucket} />
    )
    wrapper.find("li").simulate("click", { preventDefault: jest.fn() })
    expect(selectBucket).toHaveBeenCalledWith("test")
  })

  it("should highlight the selected bucket", () => {
    const wrapper = shallow(<Bucket bucket={"test"} isActive={true} />)
    expect(wrapper.find("li").hasClass("active")).toBeTruthy()
  })
})

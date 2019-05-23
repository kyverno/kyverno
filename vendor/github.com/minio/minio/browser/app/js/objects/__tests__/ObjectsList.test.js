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
import { ObjectsList } from "../ObjectsList"

describe("ObjectsList", () => {
  it("should render without crashing", () => {
    shallow(<ObjectsList objects={[]} />)
  })

  it("should render ObjectContainer for every object", () => {
    const wrapper = shallow(
      <ObjectsList objects={[{ name: "test1.jpg" }, { name: "test2.jpg" }]} />
    )
    expect(wrapper.find("Connect(ObjectContainer)").length).toBe(2)
  })

  it("should render PrefixContainer for every prefix", () => {
    const wrapper = shallow(
      <ObjectsList objects={[{ name: "abc/" }, { name: "xyz/" }]} />
    )
    expect(wrapper.find("Connect(PrefixContainer)").length).toBe(2)
  })
})

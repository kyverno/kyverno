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
import { SideBar } from "../SideBar"

jest.mock("../../web", () => ({
  LoggedIn: jest.fn(() => false).mockReturnValueOnce(true)
}))

describe("SideBar", () => {
  it("should render without crashing", () => {
    shallow(<SideBar />)
  })

  it("should not render BucketSearch for non LoggedIn users", () => {
    const wrapper = shallow(<SideBar />)
    expect(wrapper.find("Connect(BucketSearch)").length).toBe(0)
  })

  it("should call clickOutside when the user clicks outside the sidebar", () => {
    const clickOutside = jest.fn()
    const wrapper = shallow(<SideBar clickOutside={clickOutside} />)
    wrapper.simulate("clickOut", {
      preventDefault: jest.fn(),
      target: { classList: { contains: jest.fn(() => false) } }
    })
    expect(clickOutside).toHaveBeenCalled()
  })

  it("should not call clickOutside when user clicks on sidebar toggle", () => {
    const clickOutside = jest.fn()
    const wrapper = shallow(<SideBar clickOutside={clickOutside} />)
    wrapper.simulate("clickOut", {
      preventDefault: jest.fn(),
      target: { classList: { contains: jest.fn(() => true) } }
    })
    expect(clickOutside).not.toHaveBeenCalled()
  })
})

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
import { connect } from "react-redux"
import { Scrollbars } from "react-custom-scrollbars"
import * as actionsBuckets from "./actions"
import { getVisibleBuckets } from "./selectors"
import BucketContainer from "./BucketContainer"
import web from "../web"
import history from "../history"
import { pathSlice } from "../utils"

export class BucketList extends React.Component {
  componentWillMount() {
    const { fetchBuckets, setBucketList, selectBucket } = this.props
    if (web.LoggedIn()) {
      fetchBuckets()
    } else {
      const { bucket, prefix } = pathSlice(history.location.pathname)
      if (bucket) {
        setBucketList([bucket])
        selectBucket(bucket, prefix)
      } else {
        history.replace("/login")
      }
    }
  }
  render() {
    const { visibleBuckets } = this.props
    return (
      <div className="fesl-inner">
        <Scrollbars
          renderTrackVertical={props => <div className="scrollbar-vertical" />}
        >
          <ul>
            {visibleBuckets.map(bucket => (
              <BucketContainer key={bucket} bucket={bucket} />
            ))}
          </ul>
        </Scrollbars>
      </div>
    )
  }
}

const mapStateToProps = state => {
  return {
    visibleBuckets: getVisibleBuckets(state)
  }
}

const mapDispatchToProps = dispatch => {
  return {
    fetchBuckets: () => dispatch(actionsBuckets.fetchBuckets()),
    setBucketList: buckets => dispatch(actionsBuckets.setList(buckets)),
    selectBucket: (bucket, prefix) =>
      dispatch(actionsBuckets.selectBucket(bucket, prefix))
  }
}

export default connect(
  mapStateToProps,
  mapDispatchToProps
)(BucketList)

import React from 'react'
import {Col, Row} from 'react-bootstrap'
import {connect} from 'react-redux'
import Websocket from 'react-websocket'

import Sortable from 'lib/util/sortable'

const mapStateToProps = (state) => {
	return {
	}
}

@connect(mapStateToProps)
export default class Monitor extends React.Component {
	static propTypes = {
		inputs: React.PropTypes.array
	}

	state = {
		msgs: [],
		md5: '00000000000000000000000000000000',
		headers: [
			{ id: 'count', name: '#', className: 'text-right' },
			{ id: 'msg', name: 'Message' }
		]
	}

	timer = () => {
		var now = (new Date()).getTime()
		let m = this.state.msgs.map((k) => {
			if (k.timeout < now) {
				k.className = undefined
			}
			return k
		})
		this.setState({ msgs: m })
	}

	componentDidMount () {
		setInterval(this.timer, 100)
	}

	handleLogMessage = (msg) => {
		let result = JSON.parse(msg)

		let newMsg = {}
		newMsg.match = atob(result.Raw).replace(/0x[0-9a-f]{2,8}/i, '').replace(/\d/g, '').replace(/<.*@.*>/g,'')
		newMsg.msg = atob(result.Raw)
		newMsg.count = 1

		let m = this.state.msgs
		let newm = []
		let match = false

		// Walk current entries see if we are previously logged
		m.map((k) => {
			if (k.match === newMsg.match) {
				k.count++
				match = true
				let now = (new Date()).getTime()
				if (!k.txttimeout || k.txttimeout < now) {
					k.msg = newMsg.msg
					k.txttimeout = now + 200
				}
				k.timeout = now + 1000
				k.className = 'danger'
				k.red = true
				newm.unshift(k)
			}
		})
		// If not we are the first entry
		if (!match) {
			let now = (new Date()).getTime()
			newMsg.timeout = now + 500
			newMsg.txttimeout = now + 1000
			newMsg.red = true
			newMsg.className = 'danger'
			newm.unshift(newMsg)
		}
		// Append older entries
		m.map((k) => {
			if (k.match !== newMsg.match) {
				if (newm.length < 20) {
					newm.push(k)
				}
			}
		})

		this.setState({msgs: newm})
	}

	clear = () => {
		this.setState({msgs: []})
	}

	render () {
		return (<div>
			<Websocket url={'ws://dev-logging.xsnews.intern:5145/stream?md5=' + this.state.md5} onMessage={this.handleLogMessage}/>
			<Row fill>
				<Col xs={12}>
					<Sortable rows={this.state.msgs} headers={this.state.headers}/>
				</Col>
			</Row>
		</div>)
	}
}

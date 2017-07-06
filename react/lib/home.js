import React from 'react'
import {connect} from 'react-redux'

import {Col, Navbar, Nav, Row} from 'react-bootstrap'

import Monitor from 'lib/panels/monitor'

const mapStateToProps = (state) => ({
})

const Navigation = () =>
	<Navbar fluid>
		<Navbar.Header>
			<Navbar.Brand>
				<span>
					<i className='fa fa-fw fa-th-list'></i>
					&nbsp;
					GOSYSLOGD Monitor
				</span>
			</Navbar.Brand>
			<Navbar.Toggle/>
		</Navbar.Header>
		<Navbar.Collapse>
			<Nav pullRight>
				<Navbar.Brand>
					<small className='text-muted'>
						<a href='https://www.xsnews.nl' target='_blank'>Tomarus Internet Media</a>
					</small>
				</Navbar.Brand>
			</Nav>
		</Navbar.Collapse>
	</Navbar>

const Main = () =>
	<Row fill>
		<Col xs={12}><Monitor/></Col>
	</Row>

@connect(mapStateToProps)
export default class extends React.Component {
	render () {
		return (
			<div>
				<Navigation/>
				<Main/>
			</div>
		)
	}
}

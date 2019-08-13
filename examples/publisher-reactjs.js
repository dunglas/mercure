import React from 'react'
import Axios from 'axios'
import { connect } from 'react-redux'
import querystring from 'querystring'
import store from '../../store'

class MercureTest extends React.Component {
  constructor(props) {
    super(props)

    this.state = {
      // Generate token by yourself. Here, token is generated with key: mercure-secret-key
      token: 'Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJub3RpZmljYXRpb25zIl0sInB1Ymxpc2giOlsibm90aWZpY2F0aW9ucyJdfX0._JD8CB57DHocT4iLu_os-qgLKujJ7v3aOmUxtm6aCIY',
      // You must put it in a configuration file
      mercure: 'http://localhost:3000/hub'
    }

    this.handleClick = this.handleClick.bind(this)
  }

  componentDidMount() {
    store.subscribe(() => this.forceUpdate())
  }

  handleClick() {
    const { mercure, token } = this.state

    const data = querystring.stringify({
      topic: 'notifications',
      data: JSON.stringify({ id: 1, message: 'Test notification' })
    })

    Axios.post(mercure, data, {
      headers: {
        Authorization: token,
        'Content-Type': 'application/x-www-form-urlencoded'
      }
    })
  }
  
  render() {
    const { notifications } = this.props

    return (
      <div>
        <button type="button" onClick={() => this.handleClick()}>Notify!</button>
        {notifications.map(res => {
          return (
	    <div key={`notif-${res.id}`}>{res.message}</div>
          )
        })}
      </div>
    )
  }
}

const mapStateToProps = state => {
  return {
    notifications: state.mercure.notifications
  }
}

export default connect(mapStateToProps)(MercureTest)

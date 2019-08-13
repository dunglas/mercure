import React from 'react'
import Axios from 'axios'
import querystring from 'querystring'

export default class MercureTest extends React.Component {
  constructor(props) {
    super(props)

    this.state = {
      notifications: []
    }
  }

  componentDidMount() {
    const { notifications } = this.state
    const u = new URL('http://localhost:3000/hub')
    u.searchParams.append('topic', 'notifications')

    const es = new EventSource(u)
    es.onmessage = e => {
      notifications.push(JSON.parse(e.data))

      this.setState({ notifications })
    }
  }

  render() {
    const { notifications } = this.state
    const handleClick = () => {
      // Generate token by yourself. Here, token is generated with key: aVerySecretKey
      const token = 'Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJtZXJjdXJlIjp7InN1YnNjcmliZSI6WyJub3RpZmljYXRpb25zIl0sInB1Ymxpc2giOlsibm90aWZpY2F0aW9ucyJdfX0.HLLheT72BhnsuBcR2Bf3vjeLnjEfZdqa71NecStp46s'

      const data = querystring.stringify({
        topic: 'notifications',
        data: JSON.stringify({ id: 1, message: 'Test notification' })
      })

      Axios.post('http://localhost:3000/hub', data, {
        headers: {
          Authorization: token,
          'Content-Type': 'application/x-www-form-urlencoded'
        }
      })
    }

    return (
      <div style={{ padding: '40vh 40vw', color: 'black' }}>
        <button type="button" onClick={handleClick}>
          Notify!
        </button>
        {notifications && notifications.map(notification => {
          return <div key={`key-${notification.id}`}>{notification.message}</div>
        })}
      </div>
    )
  }
}

const { hubURL, topic } = JSON.parse(document.getElementById('config').textContent)

const subscribeURL = new URL(hubURL)
subscribeURL.searchParams.append('topic', topic)

const es = new EventSource(subscribeURL, { withCredentials: true })
let ul = null
es.onmessage = ({ data }) => {
    const { username, message } = JSON.parse(data)
    if (!username || !message) throw new Error('Invalid payload')

    if (!ul) {
        ul = document.createElement('ul')

        const messages = document.getElementById('messages')
        messages.innerHTML = ''
        messages.append(ul)
    }

    const li = document.createElement('li')
    li.append(document.createTextNode(`<${username}> ${message}`))
    ul.append(li)
}

document.querySelector('form').onsubmit = function (e) {
    e.preventDefault()

    const body = new URLSearchParams({
        data: JSON.stringify({ username: this.elements.username.value, message: this.elements.message.value }),
        topic,
    })
    fetch(hubURL, { method: 'POST', body, credentials: 'include' })
    this.elements.message.value = ''
    this.elements.message.focus()
}

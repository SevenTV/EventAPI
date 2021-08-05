# SevenTV EventAPI

![](https://cdn.discordapp.com/attachments/817075418640678964/871969835087704124/icon-512x512.png)

### External API Documentation

#### GET /v1/channel-emotes

This API is an example of [ServerSentEvents](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)

This Endpoint takes in query paramaters with the name `channel` as the channel name ie. `troydota` or `anatoleam`.

An example urls are:
1. `https://events.7tv.app/v1/channel-emotes?channel=troydota&channel=anatoleam`
2. `https://events.7tv.app/v1/channel-emotes?channel=troydota,anatoleam`
3. `https://events.7tv.app/v1/channel-emotes?channel=troydota+anatoleam`
4. `https://events.7tv.app/v1/channel-emotes?channel=troydota%20anatoleam`

You can request to listen up 100 channels per connection.

##### Events

1. `connection` this event is emitted when you first connect to show that the connection has been established successfully.
2. `update` this event is emitted when an update happens.
3. `heartbeat` this event is emitted to force the connection to stay open and is sent every 30 seconds.

##### JavaScript code

```js
const source = new EventSource('https://events.7tv.app/v1/channel-emotes?channel=troydota&channel=anatoleam');

source.addEventListener("connected", (e) => {
  // Should be "7tv-event-sub.v1" since this is the `v1` endpoint
  console.log(e.data); 
}, false);

source.addEventListener("message", (e) => {
  // This is a JSON payload matching the type for the specified event channel
  console.log(e.data);
}, false);

source.addEventListener("open", (e) => {
  // Connection was opened.
}, false);

source.addEventListener("ready", (e) => {
  // Connection is ready to receive events
})

source.addEventListener("error", (e) => {
  if (e.readyState === EventSource.CLOSED) {
    // Connection was closed.
  }
}, false);
```

### Internal API Documentation

#### Redis PubSub

```redis
PUBLISH "events-v1:channel-emotes:<channel-name>" "json-payload";
```
Where the JSON payload is an EmoteEventUpdate.

#### GET /health

This endpoint is entirely internal and will return a `200` if redis responds or a `5xx` status if redis does not respond.
The `X-Active-Connections` header will be set with the current number of active connections.

### JSON Payloads

#### EmoteEventUpdate

```ts
interface EmoteEventUpate {
  channel: string; // The channel this update affects
  emote_id: string; // The ID of the emote
  emote?: Emote; // An emote object. Null if the action is "REMOVE"
  name: string; // The name of the emote
  action: "ADD" | "REMOVE" | "UPDATE"; // The action done
  actor: string; // The user who caused this
}
```

1. The `channel` is the channel this event relates to.
1. The `emote_id` is an ObjectID as hex.
1. The `emote` is an Emote object.
1. The `name` is the name of the emote, respecting capitalization.
1. The `action` is either `"ADD"` or `"REMOVE"` or `"UPDATE"`.
1. The `actor` is the person who issued the triggered this event. By either adding, removing or aliasing an emote.

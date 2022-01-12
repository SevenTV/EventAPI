# SevenTV EventAPI

![7tv logo](https://cdn.discordapp.com/attachments/817075418640678964/871969835087704124/icon-512x512.png)

## External API Documentation

### GET /v1/channel-emotes

This API is an example of [ServerSentEvents](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)

This Endpoint takes in query paramaters with the name `channel` as the channel name ie. `troydota` or `anatoleam`.

An example urls are:

1. `https://events.7tv.app/v1/channel-emotes?channel=troydota&channel=anatoleam`

2. `https://events.7tv.app/v1/channel-emotes?channel=troydota,anatoleam`

3. `https://events.7tv.app/v1/channel-emotes?channel=troydota+anatoleam`

4. `https://events.7tv.app/v1/channel-emotes?channel=troydota%20anatoleam`

There is also a websocket API on this same endpoint.

If you connect to `wss://events.7tv.app/v1/channel-emotes` your connection will be handled as a websocket.

To join a channel,

```json
{
  "action": "join",
  "payload": "troydota"
}
```

To part a channel,

```json
{
  "action": "part",
  "payload": "troydota"
}
```

Upon doing either a join or a part this you will receive an event:

```json
{
  "action": "success",
  "payload": "join"
}
```

or an error

```json
{
  "action": "error",
  "payload": "<reason>"
}
```

You can also specify multiple channels, the same ways you would in the query string.

When an event is fired you will receive this:

```json
{
  "action": "update",
  "payload": "{\"channel\":\"troydota\",...}",
}
```

Every 30 seconds you will receive a ping, you do not need to respond to it.

```json
{
  "action": "ping"
}
```

The payload is a stringified json payload.

You can request to listen up 100 channels per connection.

#### Events

1. `ready` this event is emitted when the connection is fully established and ready to receive events
2. `update` this event is emitted when an update happens.
3. `heartbeat` this event is emitted to force the connection to stay open and is sent every 30 seconds.

#### JavaScript code

```js
const source = new EventSource('https://events.7tv.app/v1/channel-emotes?channel=troydota&channel=anatoleam');

source.addEventListener("ready", (e) => {
  // Should be "7tv-event-sub.v1" since this is the `v1` endpoint
  console.log(e.data); 
}, false);

source.addEventListener("update", (e) => {
  // This is a JSON payload matching the type for the specified event channel
  console.log(e.data);
}, false);

source.addEventListener("open", (e) => {
  // Connection was opened.
}, false);

source.addEventListener("error", (e) => {
  if (e.readyState === EventSource.CLOSED) {
    // Connection was closed.
  }
}, false);
```

## Internal API Documentation

### Redis PubSub

```redis
PUBLISH "events-v1:channel-emotes:<channel-name>" "json-payload";
```

Where the JSON payload is an EmoteEventUpdate.

## JSON Payloads

### EmoteEventUpdate

```ts
interface EmoteEventUpdate {
  // The channel this update affects.
  channel: string; 
  // The ID of the emote.
  emote_id: string; 
  // The name or channel alias of the emote.
  name: string; 
  // The action done.
  action: "ADD" | "REMOVE" | "UPDATE"; 
  // The user who caused this event to trigger.
  actor: string; 
  // An emote object. Null if the action is "REMOVE".
  emote?: ExtraEmoteData; 
}

interface ExtraEmoteData {
  // Original name of the emote.
  name: string;
  // The visibility bitfield of this emote.
  visibility: number;
  // The MIME type of the images.
  mime: string;
  // The TAGs on this emote.
  tags: string[];
  // The widths of the images.
  width: [number, number, number, number];
  // The heights of the images.
  height: [number, number, number, number];
  // The animation status of the emote.
  animated: boolean;
  // Infomation about the uploader.
  owner: {
    // 7TV ID of the owner.
    id: string;
    // Twitch ID of the owner.
    twitch_id: string;
    // Twitch DisplayName of the owner.
    display_name: string;
    // Twitch Login of the owner. 
    login: string;
  };
  // The first string in the inner array will contain the "name" of the URL, like "1" or "2" or "3" or "4"
  // or some custom event names we haven't figured out yet such as "christmas_1" or "halloween_1" for special versions of emotes.
  // The second string in the inner array will contain the actual CDN URL of the emote. You should use these URLs and not derive URLs
  // based on the emote ID and size you want, since in future we might add "custom styles" and this will allow you to easily update your app, 
  // and solve any future breaking changes you apps might receive due to us changing.
  urls: [[string, string]];
}

```

Example JSON

```json
{
  "channel": "troydota",
  "emote_id": "60bf2b5b74461cf8fe2d187f",
  "name": "WIDEGIGACHAD",
  "action": "ADD",
  "actor": "TroyDota",
  "emote": {
    "name": "WIDEGIGACHAD",
    "visibility": 0,
    "mime": "image/webp",
    "tags": [],
    "width": [384, 228, 144, 96],
    "height": [128, 76, 48, 32],
    "animated": true,
    "owner": {
      "id": "60af9846ebfcf7562edb10a3",
      "display_name": "noodlemanhours",
      "twitch_id": "84181465",
      "login": "noodlemanhours"
    },
  },
  "urls": [
    ["1", "https://cdn.7tv.app/emote/60bf2b5b74461cf8fe2d187f/1x"],
    ["2", "https://cdn.7tv.app/emote/60bf2b5b74461cf8fe2d187f/2x"],
    ["3", "https://cdn.7tv.app/emote/60bf2b5b74461cf8fe2d187f/3x"],
    ["4", "https://cdn.7tv.app/emote/60bf2b5b74461cf8fe2d187f/4x"],
  ]
}
```

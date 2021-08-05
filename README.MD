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
  extra_data?: ExtraEmoteData; 
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
  }
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
  "extra_data": {
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
  }
}
```

1. The `channel` is the channel this event relates to.
1. The `emote_id` is an ObjectID as hex.
1. The `emote` is an Emote object.
1. The `name` is the name of the emote, respecting capitalization.
1. The `action` is either `"ADD"` or `"REMOVE"` or `"UPDATE"`.
1. The `actor` is the person who issued the triggered this event. By either adding, removing or aliasing an emote.

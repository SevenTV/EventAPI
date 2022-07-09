# 7TV EventAPI

<!-- ![Logo](https://cdn.discordapp.com/attachments/817075418640678964/871969835087704124/icon-512x512.png) -->

The Event API allows developers to receive real-time updates on 7TV data and implement instantaneous feedback in applications. This service is used primarily to push channel emote changes to clients without requiring a manual refresh and it powers systems such as personal emotes. To get started on using this service see the documentation below.

### Supported Protocols
- [x] Server-Sent Events / HTTP
- [x] WebSocket
- [ ] Webhooks (planned)

## API Documentation

### Versions

**Base URL**: `https://events.7tv.io`

| Name |    Status    |
| ---- | :----------: |
| V3   | ✅ Supported  |
| V2   |   Skipped    |
| V1   | ⚠️ Deprecated |

### Opcodes
| Op  |     Name      | Type |                                                            Description |
| --- | :-----------: | :--- | ---------------------------------------------------------------------: |
| 0   |   Dispatch    | ⬇️    |      A standard event message, sent when a subscribed event is emitted |
| 1   |     Hello     | ⬇️    |              Received upon connecting, presents info about the session |
| 2   |   Heartbeat   | ⬇️    |                                  Ensures the connection is still alive |
| 4   |   Reconnect   | ⬇️    |                                   Server wants the client to reconnect |
| 5   |      Ack      | ⬇️    |                            Server acknowledges an action by the client |
| 6   |     Error     | ⬇️    |                                  An error occured, you should log this |
| 7   | End of Stream | ⬇️    | The server will send no further data and imminently end the connection |
| 33  |   Identify    | ⬆️    |                                           Authenticate with an account |
| 34  |    Resume     | ⬆️    |                                       Try to resume a previous session |
| 35  |   Subscribe   | ⬆️    |      Watch for changes on specific objects or sources. Don't smash it! |
| 36  |  Unsubscribe  | ⬆️    |                                             Stop listening for changes |
| 37  |    Signal     | ⬆️    |

*Legends: ⬆️ sent by client, ⬇️ sent by server*

### Server-Sent Events

TODO

### WebSocket

For apps which commonly need to add or remove event subscriptions, WebSocket is typically a better choice.

The websocket access point is `wss://events.7tv.io/v3`. The following sections will cover how to maintain and modify the connection.

#### Message Structure

WebSocket messages are composed of three root properties, `op` is the opcode, `t` is the timstamp of the message's formation, and `d` is the data payload. 

| Key |  Type  |                     Description                     |
| --- | :----: | :-------------------------------------------------: |
| op  | uint8  |               message operation code                |
| t   |  date  | timestamp of the message's formation in unix millis |
| d   | object |                generic data payload                 |

#### Connection

Upon establishing a connection, you will receive a `[1] HELLO` opcode. The payload contains the following data

| Key                |  Type  |                                Description                                |
| ------------------ | :----: | :-----------------------------------------------------------------------: |
| heartbeat_interval | int32  |              interval in milliseconds between each heartbeat              |
| session_id         | string | unique token for this session, used for resuming and mutating the session |

#### Heartbeat

The server will send periodic heartbeats at the interval specified in the Hello payload. If heartbeats are missed for 3 cycles, the connection can be considered dead (i.e due to an error or network issue) and you should reconnect.

#### Managing subscriptions

A subscription consists of a **type** and a **condition**. This is where you can choose exactly what kind of data your application needs.

##### Subscription Types
| Type                   | Kind                   |
| ---------------------- | :--------------------- |
| System - Announcement  | system.announcement    |
| Create Emote           | emote.create           |
| Update Emote           | emote.update           |
| Delete Emote           | emote.delete           |
| Create Emote Set       | emote_set.create       |
| Update Emote Set       | emote_set.update       |
| Delete Emote Set       | emote_set.delete       |
| Create User            | user.create            |
| Update User            | user.update            |
| Delete User            | user.delete            |
| Add User Connection    | user.add_connection    |
| Update User Connection | user.update_connection |
| Delete User Connection | user.delete_connection |

If you'd like to receive all events about an object, it is also possible to use an asterisk symbol as a wildcard. For example, using the type `emote.*` will subscribe to each of `emote.create`, `emote.update` and `emote.delete`.

##### Subscribing

To set up a subscription, you may send the opcode `[35] SUBSCRIBE` and these fields

| Key       |    Type    |          Description          |
| --------- | :--------: | :---------------------------: |
| type      |   string   |       subscription type       |
| condition | string map | filter messages by conditions |

> Example
```json
{
    "op": 35,
    "d": {
        "type": "emote_set.update",
        "condition": {
            // valid fields in the condition depend on the subscription type
            // though in most cases except creations, object_id is acceptable
            // to filter for a specific object.

            "object_id": "603b7c7496832ffa78522da5" 
        }
    }
}
```

You are allowed to subscribe multiple times to the same type, however, duplicated conditions will result in a disconnect with code `4009 Already Subscribed`.

<details>
<summary>Legacy v1</summary>

*This version is deprecated and will eventually be discontinued. Please update to v3*

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
</details>
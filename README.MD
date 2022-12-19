# 7TV EventAPI

<!-- ![Logo](https://cdn.discordapp.com/attachments/817075418640678964/871969835087704124/icon-512x512.png) -->

The Event API allows developers to receive real-time updates on 7TV data and implement instantaneous feedback in applications. This service is used primarily to push channel emote changes to clients without requiring a manual refresh and it powers systems such as personal emotes. To get started on using this service see the documentation below.

- [7TV EventAPI](#7tv-eventapi)
    - [Supported Protocols](#supported-protocols)
  - [API Documentation](#api-documentation)
    - [Versions](#versions)
    - [Opcodes](#opcodes)
    - [Close codes](#close-codes)
    - [Payloads](#payloads)
      - [Dispatch (0)](#dispatch-0)
        - [ChangeMap](#changemap)
        - [ChangeField](#changefield)
      - [Hello (1)](#hello-1)
      - [Heartbeat](#heartbeat)
      - [Ack (5)](#ack-5)
      - [Subscribe (35)](#subscribe-35)
      - [Unsubscribe (36)](#unsubscribe-36)
    - [Subscription types](#subscription-types)
        - [Subscription Types](#subscription-types-1)
    - [EventStream / Server-Sent Events (HTTP)](#eventstream--server-sent-events-http)
      - [Message Structure (EventStream)](#message-structure-eventstream)
      - [Connection (EventStream)](#connection-eventstream)
        - [Inline Event Subscriptions (EventStream)](#inline-event-subscriptions-eventstream)
      - [Managing subscriptions (EventStream)](#managing-subscriptions-eventstream)
      - [Acks (EventStream)](#acks-eventstream)
      - [Dispatches (EventStream)](#dispatches-eventstream)
    - [WebSocket](#websocket)
      - [Message Structure (WebSocket)](#message-structure-websocket)
      - [Connection (WebSocket)](#connection-websocket)
      - [Heartbeat (WebSocket)](#heartbeat-websocket)
      - [Managing subscriptions (WebSocket)](#managing-subscriptions-websocket)
        - [Subscribing (WebSocket)](#subscribing-websocket)
        - [Unsubscribing (WebSocket)](#unsubscribing-websocket)
      - [Acks (WebSocket)](#acks-websocket)
      - [Dispatches (WebSocket)](#dispatches-websocket)

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

Operation codes are used to identify what kind of payload an event contains.

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

### Close codes

Close codes provide a reason for the closure of a connection to help clients understand what happened.

| Code | Name                   |                                Description                                |
| ---- | :--------------------- | :-----------------------------------------------------------------------: |
| 4000 | Server Error           |                   an error occured on the server's end                    |
| 4001 | Unknown Operation      |                   the client sent an unexpected opcode                    |
| 4002 | Invalid Payload        |            the client sent a payload that couldn't be decoded             |
| 4003 | Auth Failure           |                the client unsucessfully tried to identify                 |
| 4004 | Already Identified     |                    the client wanted to identify again                    |
| 4005 | Rate Limited           |                     the client is being rate-limited                      |
| 4006 | Restart                |         the server is restarting and the client should reconnect          |
| 4007 | Maintenance            |      the server is in maintenance mode and not accepting connections      |
| 4008 | Timeout                |                     the client was idle for too long                      |
| 4009 | Already Subscribed     |              the client tried to subscribe to an event twice              |
| 4010 | Not Subscribed         | the client tried to unsubscribe from an event they weren't subscribing to |
| 4011 | Insufficient Privilege |      the client did something that they did not have permission for       |

### Payloads

#### Dispatch (0)

| Key  |               Type               |   Description    |
| ---- | :------------------------------: | :--------------: |
| type | [EventType](#subscription-types) |  the event type  |
| body |     [ChangeMap](#changemap)      | detailed changes |

##### ChangeMap
| Key         |             Type              |                                Description                                |
| ----------- | :---------------------------: | :-----------------------------------------------------------------------: |
| id          |           ObjectID            |                              the object's ID                              |
| kind        |             int8              |                              the object kind                              |
| contextual? |             bool              | if true, this event represents a change local only to the current session |
| actor       |             User              |                  the user responsible for these changes                   |
| added?      | [][ChangeField](#changefield) |                          a list of added fields                           |
| updated?    | [][ChangeField](#changefield) |                         a list of updated fields                          |
| removed?    | [][ChangeField](#changefield) |                         a list of removed fields                          |
| pushed?     | [][ChangeField](#changefield) |                    a list of items pushed to an array                     |
| pulled?     | [][ChangeField](#changefield) |                   a list of items pulled from an array                    |

##### ChangeField

| Key       |                     Type                     |                                        Description                                        |
| --------- | :------------------------------------------: | :---------------------------------------------------------------------------------------: |
| key       |                    string                    |                                    the key in context                                     |
| index     |                     int?                     | if the field is an array, this is the index of the item within the array that was updated |
| nested¹   |                     bool                     |                  if true, this means the current value is nested deeper                   |
| old_value |                object or nil                 |                                    the previous value                                     |
| value     | object, [][ChangeField](#changefield) or nil |                                       the new value                                       |

**¹** _the `value` field will always be `[]ChangeField` when `nested` is `true`._

#### Hello (1)

| Key                |  Type  |                                Description                                |
| ------------------ | :----: | :-----------------------------------------------------------------------: |
| heartbeat_interval | uint32 |              interval in milliseconds between each heartbeat              |
| session_id         | string | unique token for this session, used for resuming and mutating the session |
| subscription_limit | int32  |     the maximum amount of subscriptions this connection can initiate      |

#### Heartbeat

| Key   |  Type  |           Description           |
| :---- | :----: | :-----------------------------: |
| Count | uint64 | The amount of heartbeats so far |

#### Ack (5)

|   Key   |  Type  |                Description                |
| :-----: | :----: | :---------------------------------------: |
| command | string | the acknowledged sent opcode in text form |
|  data   |  echo  |        the data sent by the client        |

#### Subscribe (35)

| Key       |    Type    |          Description          |
| --------- | :--------: | :---------------------------: |
| type      |   string   |       subscription type       |
| condition | string map | filter messages by conditions |

> Example
```jsonc
{
    "op": 35,
    "d": {
        "type": "emote_set.update",
        "condition": {
            // valid fields in the condition depend on the subscription type
            // though in most cases except creations, object_id is acceptable
            // to filter for a specific object.

            "object_id": "62cdd34e72a832540de95857"
        }
    }
}
```

#### Unsubscribe (36)

| Key        |    Type    |          Description          |
| ---------- | :--------: | :---------------------------: |
| type       |   string   |       subscription type       |
| condition? | string map | filter messages by conditions |

```jsonc
{
  "op": 36, 
  "type": "emote_set.update",
  "condition": {
    "object_id": "62cdd34e72a832540de95857"
  }
}
```

### Subscription types

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
| Create Cosmetic        | cosmetic.create        |
| Update Cosmetic        | cosmetic.update        |
| Delete Cosmetic        | cosmetic.delete        |
| Create Entitlement     | entitlement.create     |
| Update Entitlement     | entitlement.update     |
| Delete Entitlement     | entitlement.delete     |

If you'd like to receive all events about an object, it is also possible to use an asterisk symbol as a wildcard. For example, using the type `emote.*` will subscribe to each of `emote.create`, `emote.update` and `emote.delete`.

---

### EventStream / Server-Sent Events (HTTP)

EventStream, or [Server-Sent Events (SSE)](https://en.wikipedia.org/wiki/Server-sent_events) is the recommended way to use the EventAPI.
This is a one-way stream which operates via a regular HTTP connection rather than a different protocol such as WebSocket.

The EventStream access point is [`https://events.7tv.io/v3`](https://events.7tv.io/v3). The following sections will cover how to initiate and modify the connection.

#### Message Structure (EventStream)

In a browser environment, the `EventSource` interface exists to automatically handle an EventStream endpoint.

Otherwise, to read and parse this data refer to the [spec for Server-Sent Events](https://html.spec.whatwg.org/multipage/server-sent-events.html).

Opcodes are sent in their text form as the event's type, and the data will always be JSON.

#### Connection (EventStream)

Upon opening the connection, you will receive a [`[1] HELLO`](#hello-1) event.

##### Inline Event Subscriptions (EventStream)

It is possible to add subscriptions directly in the connection string, by appending a `@` to the URL followed by a URL-encoded string with the following syntax,

`{type}<{condition1}={value1},{condition2}={value2}>,...`

where `type` is a [subscription type](#subscription-types), then wrapped between brackets (`<...>`) are a list of conditions. 
Separate each subscription by a comma. The same type can be subscribed to multiple times with different conditions.

The entire inline subscription string **must be URL-encoded**.

Full examples

```
Subscribe to updates on a specific emote set
: GET https://events.7tv.io/v3@emote_set.update<object_id=62cdd34e72a832540de95857>

Subscribe to all entitlement and cosmetic events on a specific channel
: GET https://events.7tv.io/v3@entitlement.*<host_id=60867b015e01df61570ab900,connection_id=1234>,cosmetic.*<host_id=60867b015e01df61570ab900,connection_id=1234>
```

At this time, it is possible to subscribe with no conditions and effectively turn the connection into a Firehose, however this may be restricted in the future.

#### Managing subscriptions (EventStream)

Adding or removing a subscription is done by sending a request via a REST endpoint and passing your session ID to authenticate.

The endpoint for managing subscriptions is currently unavailable. You may use [Inline Event Subscriptions](#inline-event-subscriptions-eventstream) in the meantime.

#### Acks (EventStream)

An ack will be sent when the session is mutated, such as when subscribing or unsubscribing. ACKs are also sent when using [Inline Event Subscriptions](#inline-event-subscriptions-eventstream) immediately after the session becomes ready. This can be used to confirm the validity of the inline subscription string.

Handling ACKs is optional, but allows an implementation to confirm that the server accepted their input.

#### Dispatches (EventStream)

Once subscriptions are active, you will receive [`[0] DISPATCH`](#dispatch-0) events 

---

### WebSocket

For apps which must issue commands frequently, WebSocket may be a better choice.

The websocket access point is `wss://events.7tv.io/v3`. The following sections will cover how to maintain and modify the connection.

#### Message Structure (WebSocket)

WebSocket messages are sent in JSON format and are composed of three root properties, `op` is the opcode, `t` is the timestamp of the message's formation, and `d` is the data payload. 

| Key |  Type  |                     Description                     |
| --- | :----: | :-------------------------------------------------: |
| op  | uint8  |               message operation code                |
| t   |  date  | timestamp of the message's formation in unix millis |
| d   | object |                generic data payload                 |

#### Connection (WebSocket)

Upon establishing a connection, you will receive a [`[1] HELLO`](#hello-1) event.

#### Heartbeat (WebSocket)

The server will send periodic heartbeats at the interval specified in the Hello payload. If heartbeats are missed for 3 cycles, the connection can be considered dead (i.e due to an error or network issue) and you should reconnect.

#### Managing subscriptions (WebSocket)

A subscription consists of a **type** and a **condition**. This is where you can choose exactly what kind of data your application needs.

Valid subscription types can be found [here](#subscription-types).

##### Subscribing (WebSocket)

To set up a subscription, you may send the opcode [`[35] SUBSCRIBE`](#subscribe-35) and these fields

You are allowed to subscribe multiple times to the same type, however, duplicated conditions will result in a disconnect with code `4009 Already Subscribed`.

##### Unsubscribing (WebSocket)

When an event source is no longer needed, you should unsubscribe from it to avoid receiving unnecessary data with [`[36] UNSUBSCRIBE`](#unsubscribe-36).

It is also possible to unsubscribe from an entire event type at once by leaving the condition field empty.

#### Acks (WebSocket)

An ack will be sent when a command is successfully executed, such as subscribing or unsubscribing.
Handling ACKs is optional, but allows an implementation to confirm that the server accepted their input.

#### Dispatches (WebSocket)

Once subscriptions are active, you may start receiving messages with the [`[0] DISPATCH`](#dispatch-0) opcode. These events notify of changes to an object and allow you to keep your state up to date.

---

[View legacy v1 documentation](legacy_docs.md)
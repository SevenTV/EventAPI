# 7TV EventAPI

<!-- ![Logo](https://cdn.discordapp.com/attachments/817075418640678964/871969835087704124/icon-512x512.png) -->

The Event API allows developers to receive real-time updates on 7TV data and implement instantaneous feedback in applications. This service is used primarily to push channel emote changes to clients without requiring a manual refresh and it powers systems such as personal emotes. To get started on using this service see the documentation below.

- [7TV EventAPI](#7tv-eventapi)
    - [Supported Protocols](#supported-protocols)
  - [API Documentation](#api-documentation)
    - [Versions](#versions)
    - [Opcodes](#opcodes)
    - [Server-Sent Events](#server-sent-events)
    - [WebSocket](#websocket)
      - [Message Structure](#message-structure)
      - [Connection](#connection)
      - [Heartbeat](#heartbeat)
      - [Managing subscriptions](#managing-subscriptions)
        - [Subscription Types](#subscription-types)
        - [Subscribing](#subscribing)

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

---

[View legacy v1 documentation](legacy_docs.md)
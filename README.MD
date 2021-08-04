# SevenTV EventAPI

![](https://cdn.discordapp.com/attachments/817075418640678964/871969835087704124/icon-512x512.png)

### External API Documentation
#### GET /v1?channel=channel_name&channel=channel_name
This API [ServerSide Events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)

An example url is `https://events.7tv.app/v1?channel=troydota&channel=anatoleam`
You can request to listen up 100 channels per connection.

##### JavaScript code

```js
const source = new EventSource('https://events.7tv.app/v1?channel=troydota&channel=anatoleam');

source.addEventListener("connected", (e) => {
  // Should be "7tv-event-sub.v1" since this is the `v1` endpoint
  console.log(e.data); 
}, false);

source.addEventListener("update", (e) => {
  // This is a JSON payload of the type `EmoteEventUpdate`
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
PUBLISH "user:channel-name:emotes" "json-payload";
```
Where the JSON payload is an EmoteEventUpdate.
### JSON Payloads
#### EmoteEventUpdate

```json
{
  "channel": string,
	"emote_id": string,
	"name": string,
	"action": "removed" or "added",
	"author": string
}
```
1. The `channel` is the channel this event relates to.
2. The `emote_id` is an ObjectID as hex.
3. The `name` is the name of the emote, respecting capitalization.
4. The `action` is either `"removed"` or `"added"`.
5. The `author` is the person who issued the triggered this event. By either adding, removing or aliasing an emote.

If an emote is `aliased` a `remove` event will be fired, removing the old emote name followed by an `added` event adding the new emote name.

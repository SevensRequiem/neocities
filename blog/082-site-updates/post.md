---
id: 82
title: "Site Updates!"
date: 2025-09-03T11:56:08Z
tags: []
author: "requiem"
hex: "68b82cd8b1e4617e1104d291"
quote: "The Best is yet to Come!"
image: "image.webp"
---

it's been a while since i've made a tech post, i believe. at least one that was directly technical and had no mental/personal drama mixed in! i need to finish the blog category system, its... lacking. right now i only have a simple json string for tags, and i manually input "Tag1, Tag2, Tag3" on the admin page, as a raw string... that is redundant and hastily made! i need to have a struct, say:
type Tag struct {
  Tech bool
  Mental bool
  Personal bool
  Business bool
  Occult bool
  Programming bool
  NSFW / PTSD bool
}
and either dynamically set them with AI, or manually on the admin page... so many improvements need to be made...
todo:
- category rework
- v1.6 + new theme
- fix markdown for blog posts and comments
- clean up blog posts and comments
- finish the new achan theme
- integrate discord notifs for moderation
- remove visitor log spam (its at like 150k documents in the database.... 10+ documents per connection with alot of info....) its redundant
- rss feed + home page news?
- automatic backup of chat + blog
- automatic writing of neocities port?
- add images to x/twitter posts, maybe AI commentary on what the post is

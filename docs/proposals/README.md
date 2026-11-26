# Proposals

A proposal is a document that describes a change that should be made to CAPG. Generally speaking a proposal is 
required when what needs to be implemented needs more thought and discussion than a single PR, though there are no
hard and fast rules. In particular, if you find yourself choosing between lots of alternatives, or the design you've 
chosen introduces new concepts into CAPG it's probably a good idea to reach for a proposal first.

## How To Start

To write a proposal, the easiest thing to do is copy the [template](./YYYYMMDD-template.md) and then fill in the date
and change `template` to a descriptive title. Once the proposal is written it should be raised as a PR so it can 
be discussed and revised before being merged and implemented.

In the template there is a `status` field that shows the state of the proposal, this should be kept up to date as much
as possible. The possible values are listed in the template.

## Archiving Proposals

When proposals reach a terminal state (`implemented`, `rejected`, `withdrawn` or `replaced`) they should be moved 
into [archived](./archived). This keeps the set of live proposals as those in the main folder but allows others to 
follow the history of the implementation of ideas.

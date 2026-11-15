# Decisions answers in a transcript

The transcript uses the typed answerOrigin contract in decisions-answer-origin.md. It never recognizes a Decisions reply from its prose. Legacy messages and unsupported metadata retain the ordinary delivered-message presentation.

Each answer card shows only the question snapshots answered in that reply, in their recorded order, with the person's answers and any As Recommended marks. A fixed-height scrolling body keeps large answers inside the compact block. Its request link has an address in Decisions and opens a read-only lookup, including after closure. The lookup exposes no answer or dismissal controls and never marks anything read. An unavailable lookup explains that the answer remains in the transcript.

The decisionAnswer arrival marker reserves the same card height before delivery. Replacing that placeholder keeps subsequent transcript rows in place. Appending an answer follows the transcript's existing scrolling policy; reading above the end keeps the reading position. The composer and window controls retain their geometry.

The waiting row itself remains attached when its card becomes a delivered message. Its children and message metadata change together, preserving the row's position and the browser's remembered size. A Decisions row also uses the card's fixed height as its initial intrinsic size, so a row that has not been rendered does not temporarily use the generic 120-pixel estimate. The card and its estimate share one CSS value.

The remembered size belongs to the element, as documented in [CSS Sizing, Last Remembered Size](https://www.w3.org/TR/css-sizing-4/#last-remembered). The explicit estimate also covers delivery before that size has been recorded. The geometry test continues to require equal row heights and unchanged later rows. It must not wait away a visible displacement.

Engine owns origin persistence, delivery and replay metadata, the arrival marker and DecisionByID. Front End owns the card, the read-only view and browser checks. The architectural principles remain at C:\Users\paul\OneDrive\Documents\Architectural Principles.md.

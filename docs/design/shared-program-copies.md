# Program copies in Settings and Updates

The Set-aside Programs card has one controller and one DOM node. Settings uses a single window with mutually exclusive tabs, so its original location and the optional Updates location share selections, protected rows and removal results. Showing either location places the existing card there and rereads the same service. A missing Updates service leaves the original Settings section available on its own.

Updates places its slot after Running Copies and Agent Connections. Refreshing installed and running versions also refreshes program copies, including the existing installation completion path. The card retains its fixed scrolling areas and obsolete rows. List changes cannot move its surrounding sections or earlier rows.

The existing ListSetAsidePrograms and RemoveSetAsidePrograms methods retain all safety decisions. No new host method or removal logic is needed. The architectural principles remain at C:\Users\paul\OneDrive\Documents\Architectural Principles.md.

Feature: Node Basics

Scenario: Starting the node
  Given a node bafzqai123
  When I start it
  Then the PeerID is bafzqai123

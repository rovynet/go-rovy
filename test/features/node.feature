Feature: Node Basics

  Background:
    Given a keyfile named 'testone.toml'
      """
      PrivateKey = 'mloUFLqeJND3KizotkvrQf8vA/i0DeLCfCylpBgpnpu0'
      PeerID = 'bafzqaid26tco5uc5y22nhd6xuglvbmij5yx6iok7pzrgzf3mz3m6mellee'
      IPAddr = 'fc89:54a1:b598:365f:357:d6a:b108:db7b'
      """
    And node 'A' from keyfile 'testone.toml'

  Scenario: PeerID and IPAddress
    When a 'info' call on 'A' is successful
    Then response value 'PeerID' is 'bafzqaid26tco5uc5y22nhd6xuglvbmij5yx6iok7pzrgzf3mz3m6mellee'
    And response value 'IPAddress' is 'fc89:54a1:b598:365f:357:d6a:b108:db7b'

  Scenario: Starting and stopping
    When I start node 'A'
    Then node 'A' is running
    When I stop node 'A'
    Then node 'A' is not running
    When I start node 'A'
    Then node 'A' is running
    When I stop node 'A'
    Then node 'A' is not running

Feature: Node Basics

Background:
  Given a keyfile named 'testone.toml'
    """
    PrivateKey = 'mloUFLqeJND3KizotkvrQf8vA/i0DeLCfCylpBgpnpu0'
    PeerID = 'bafzqaid26tco5uc5y22nhd6xuglvbmij5yx6iok7pzrgzf3mz3m6mellee'
    IPAddr = 'fc89:54a1:b598:365f:357:d6a:b108:db7b'
    """
  And a keyfile named 'testtwo.toml'
    """
    PrivateKey = 'mvU3RPb32jRfGwF1gooSGv/kUSgHmkhZ2AOHtMH50Zfk'
    PeerID = 'bafzqaid64hxsipxirgje4jhrxrbfzwhr7je6oovzndfmrbzsosahsouhey'
    IPAddr = 'fc60:c1b7:76b8:4521:6d75:e0ac:f8ad:b5f4'
    """
  And a keyfile named 'testthree.toml'
    """
    PrivateKey = 'mokObwvfFYz7W9G+etq6u+T6e7KaGJZbT/aUdCvRBmlc'
    PeerID = 'bafzqaiegktgah3svfe6wmsqtvnxsputb3k2dbyl7dqpjjqdqvaga4vr3em'
    IPAddr = 'fc42:19b0:944:d4d1:673d:2ae:b20:a91b'
    """

Scenario: Starting the node
  Given node 'A' from keyfile 'testone.toml'
  When I start node 'A'
  Then the PeerID of 'A' is 'bafzqaid26tco5uc5y22nhd6xuglvbmij5yx6iok7pzrgzf3mz3m6mellee'
  And the IP of 'A' is 'fc89:54a1:b598:365f:357:d6a:b108:db7b'

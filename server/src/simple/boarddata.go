package simple

func NewBase45Board() Board {
    return Board{
        Name: "Base45",
        Cities: []City{
            City{
                Id: 0,
                Name: "Groningen",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                        Points: 1,
                    },
                    Office{
                        Shape: DiscShape,
                        Priviledge: OrangePriviledge,
                    },
                },
                Award: DiscsAward,
            },
            City{
                Id: 1,
                Name: "Emden",
                Offices: []Office{
                    Office{
                        Shape: DiscShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                },
            },
            City{
                Id: 2,
                Name: "Stade",
                Offices: []Office{
                    Office{
                        Shape: DiscShape,
                        Priviledge: WhitePriviledge,
                    },
                },
                Award: PriviledgeAward,
            },
            City{
                Id: 3,
                Name: "Hamburg",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: BlackPriviledge,
                    },
                },
            },
            City{
                Id: 4,
                Name: "Lubeck",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                        Points: 1,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                },
                Award: BagsAward,
            },
            City{
                Id: 5,
                Name: "Kampen",
                Offices: []Office{
                    Office{
                        Shape: DiscShape,
                        Priviledge: OrangePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: BlackPriviledge,
                    },
                },
            },
            City{
                Id: 6,
                Name: "Bremen",
                Offices: []Office{
                    Office{
                        Shape: DiscShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                },
            },
            City{
                Id: 7,
                Name: "Luneburg",
                Offices: []Office{
                    Office{
                        Shape: DiscShape,
                        Priviledge: OrangePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: BlackPriviledge,
                    },
                },
            },
            City{
                Id: 8,
                Name: "Verleberg",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                    Office{
                        Shape: DiscShape,
                        Priviledge: BlackPriviledge,
                    },
                },
            },
            City{
                Id: 9,
                Name: "Arnheim",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: DiscShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                },
                BonusTerminus: true,
            },
            City{
                Id: 10,
                Name: "Osnabruck",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: BlackPriviledge,
                    },
                },
            },
            City{
                Id: 11,
                Name: "Munster",
                Offices: []Office{
                    Office{
                        Shape: DiscShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                },
            },
            City{
                Id: 12,
                Name: "Minden",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: BlackPriviledge,
                    },
                },
            },
            City{
                Id: 13,
                Name: "Hannover",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                },
            },
            City{
                Id: 14,
                Name: "Hildesheim",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: BlackPriviledge,
                    },
                },
            },
            City{
                Id: 15,
                Name: "Brunswick",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                },
            },
            City{
                Id: 16,
                Name: "Stendal",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: DiscShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                },
                BonusTerminus: true,
            },
            City{
                Id: 17,
                Name: "Duisburg",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                },
            },
            City{
                Id: 18,
                Name: "Dortmund",
                Offices: []Office{
                    Office{
                        Shape: DiscShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                },
            },
            City{
                Id: 19,
                Name: "Baderborn",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: DiscShape,
                        Priviledge: BlackPriviledge,
                    },
                },
            },
            City{
                Id: 20,
                Name: "Goslar",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: BlackPriviledge,
                    },
                },
            },
            City{
                Id: 21,
                Name: "Magdeburg",
                Offices: []Office{
                    Office{
                        Shape: DiscShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                },
            },
            City{
                Id: 22,
                Name: "Coellen",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                        Points: 1,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                },
                Coellen: CoellenTable{
                    Spots: []CoellenSpot{
                        CoellenSpot{
                            Priviledge: WhitePriviledge,
                            Points: 7,
                        },
                        CoellenSpot{
                            Priviledge: OrangePriviledge,
                            Points: 8,
                        },
                        CoellenSpot{
                            Priviledge: PurplePriviledge,
                            Points: 9,
                        },
                        CoellenSpot{
                            Priviledge: BlackPriviledge,
                            Points: 11,
                        },
                    },
                },
                Award: CoellenAward,
            },
            City{
                Id: 23,
                Name: "Warburg",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                },
            },
            City{
                Id: 24,
                Name: "Gottingen",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: DiscShape,
                        Priviledge: WhitePriviledge,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: PurplePriviledge,
                    },
                },
                Award: ActionsAward,
            },
            City{
                Id: 25,
                Name: "Quedlinburg",
                Offices: []Office{
                    Office{
                        Shape: DiscShape,
                        Priviledge: OrangePriviledge,
                    },
                    Office{
                        Shape: DiscShape,
                        Priviledge: PurplePriviledge,
                    },
                },
            },
            City{
                Id: 26,
                Name: "Halle",
                Offices: []Office{
                    Office{
                        Shape: CubeShape,
                        Priviledge: WhitePriviledge,
                        Points: 1,
                    },
                    Office{
                        Shape: CubeShape,
                        Priviledge: OrangePriviledge,
                    },
                },
                Award: KeysAward,
            },
        },
        Routes: []Route{
            Route{
                Id: 0,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 0,
                RightCityId: 1,
            },
            Route{
                Id: 1,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 1,
                RightCityId: 2,
            },
            Route{
                Id: 2,
                Spots: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                LeftCityId: 1,
                RightCityId: 10,
            },
            Route{
                Id: 3,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 2,
                RightCityId: 3,
            },
            Route{
                Id: 4,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 3,
                RightCityId: 4,
            },
            Route{
                Id: 5,
                Spots: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                LeftCityId: 3,
                RightCityId: 6,
            },
            Route{
                Id: 6,
                Spots: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                LeftCityId: 3,
                RightCityId: 7,
            },
            Route{
                Id: 7,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 5,
                RightCityId: 9,
            },
            Route{
                Id: 8,
                Spots: []Piece{Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}},
                LeftCityId: 5,
                RightCityId: 10,
            },
            Route{
                Id: 9,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                StartToken: true,
                LeftCityId: 6,
                RightCityId: 10,
            },
            Route{
                Id: 10,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 6,
                RightCityId: 12,
            },
            Route{
                Id: 11,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 6,
                RightCityId: 13,
            },
            Route{
                Id: 12,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                StartToken: true,
                LeftCityId: 7,
                RightCityId: 8,
            },
            Route{
                Id: 13,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 7,
                RightCityId: 13,
            },
            Route{
                Id: 14,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 8,
                RightCityId: 16,
            },
            Route{
                Id: 15,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 9,
                RightCityId: 11,
            },
            Route{
                Id: 16,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 9,
                RightCityId: 17,
            },
            Route{
                Id: 17,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 11,
                RightCityId: 12,
            },
            Route{
                Id: 18,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 12,
                RightCityId: 13,
            },
            Route{
                Id: 19,
                Spots: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                LeftCityId: 12,
                RightCityId: 15,
            },
            Route{
                Id: 20,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 12,
                RightCityId: 19,
            },
            Route{
                Id: 21,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 14,
                RightCityId: 19,
            },
            Route{
                Id: 22,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                StartToken: true,
                LeftCityId: 14,
                RightCityId: 20,
            },
            Route{
                Id: 23,
                Spots: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                LeftCityId: 15,
                RightCityId: 16,
            },
            Route{
                Id: 24,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 16,
                RightCityId: 21,
            },
            Route{
                Id: 25,
                Spots: []Piece{Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}},
                LeftCityId: 17,
                RightCityId: 18,
            },
            Route{
                Id: 26,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 18,
                RightCityId: 19,
            },
            Route{
                Id: 27,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 19,
                RightCityId: 23,
            },
            Route{
                Id: 28,
                Spots: []Piece{Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}},
                LeftCityId: 20,
                RightCityId: 21,
            },
            Route{
                Id: 29,
                Spots: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                LeftCityId: 20,
                RightCityId: 25,
            },
            Route{
                Id: 30,
                Spots: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                LeftCityId: 22,
                RightCityId: 23,
            },
            Route{
                Id: 31,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 23,
                RightCityId: 24,
            },
            Route{
                Id: 32,
                Spots: []Piece{Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}},
                LeftCityId: 24,
                RightCityId: 25,
            },
            Route{
                Id: 33,
                Spots: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                Bumped: []Piece{Piece{}, Piece{}, Piece{}, Piece{}},
                LeftCityId: 25,
                RightCityId: 26,
            },
        },
    }
}

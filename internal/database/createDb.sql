create table if not exists Manga (
    ID integer not null primary key,
    Title text,
    TimeStampUnixEpoch integer not null,
    Thumbnail blob null,
    LatestAvailableChapter text
);

create table if not exists Chapter (
    ID integer not null primary key,
    MangaID integer not null,
    Url text not null,
    Name text null,
    Number text null,
    TimeStampUnixEpoch integer not null,
    foreign key(MangaID) references Manga(ID)
);

create table if not exists Setting (
    Name text not null primary key,
    Value text,
    DefaultValue text not null
);
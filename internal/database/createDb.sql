create table if not exists Manga (
    ID integer not null primary key,
    Title text,
    TimeStampUnixEpoch int
);

create table if not exists Chapter (
    ID integer not null primary key,
    MangaID integer not null,
    Url text not null,
    Name text null,
    Number int not null,
    TimeStampUnixEpoch int,
    foreign key(MangaID) references Manga(ID)
);
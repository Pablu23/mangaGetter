drop table if exists Chapter;
drop table if exists Manga;

create table if not exists Manga
(
    ID                 integer not null primary key autoincrement,

    Provider           integer not null,

    Rating             integer,
    Title              text,
    TimeStampUnixEpoch integer,
    Thumbnail          blob
);

create table if not exists Chapter
(
    ID                 integer not null primary key autoincrement,
    MangaID            integer not null,

    InternalIdentifier text not null,
    Url                text    not null,
    Name               text    null,
    Number             integer not null,
    TimeStampUnixEpoch integer,
    foreign key (MangaID) references Manga (ID)
);

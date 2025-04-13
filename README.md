# Repository Coverage

[Full report](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

| Name                                                  |    Stmts |     Miss |   Cover |   Missing |
|------------------------------------------------------ | -------: | -------: | ------: | --------: |
| src/posting/\_\_init\_\_.py                           |        6 |        0 |    100% |           |
| src/posting/\_\_main\_\_.py                           |       80 |       47 |     41% |22-29, 33-42, 65-73, 81-93, 107-131, 144, 154 |
| src/posting/\_start\_time.py                          |        2 |        0 |    100% |           |
| src/posting/app.py                                    |      675 |      163 |     76% |216, 220, 282-283, 291-298, 323, 329-344, 350-356, 365, 367, 369, 386-387, 397-399, 406, 429-430, 440, 461-462, 469-494, 514, 526, 595, 603, 620-636, 758-770, 778-788, 865, 875-876, 889-918, 929, 965, 1095, 1101-1119, 1129-1154, 1160-1180, 1190-1191, 1194, 1202, 1220-1224, 1237, 1240, 1243, 1254, 1257, 1260-1262, 1281-1282, 1307-1319, 1330, 1340, 1351-1359, 1368-1369, 1387-1392, 1398-1399, 1424-1425 |
| src/posting/auth.py                                   |        8 |        3 |     62% |  8, 11-12 |
| src/posting/collection.py                             |      343 |       78 |     77% |30-39, 53, 119, 210-211, 225-228, 231-243, 282-286, 303, 415-416, 422-455, 470-471, 500-501, 520-527 |
| src/posting/commands.py                               |       62 |        5 |     92% |7, 28, 95, 97, 147 |
| src/posting/config.py                                 |      118 |        2 |     98% |  231, 248 |
| src/posting/exit\_codes.py                            |        1 |        0 |    100% |           |
| src/posting/files.py                                  |       63 |       27 |     57% |68, 92-94, 110-146 |
| src/posting/help\_data.py                             |        7 |        0 |    100% |           |
| src/posting/help\_screen.py                           |       56 |        4 |     93% |   145-152 |
| src/posting/highlight\_url.py                         |        0 |        0 |    100% |           |
| src/posting/highlighters.py                           |       51 |        2 |     96% |    51, 64 |
| src/posting/importing/curl.py                         |      146 |       71 |     51% |122, 125, 152, 159, 172-181, 189-259, 264-301 |
| src/posting/importing/open\_api.py                    |      227 |      102 |     55% |50-52, 66, 96, 113-114, 123, 135-136, 155, 158-165, 168-172, 175-178, 216-221, 226-228, 231-234, 237-260, 263-280, 330, 349, 372-388, 391-400, 409-416 |
| src/posting/jump\_overlay.py                          |       71 |       17 |     76% |13, 65, 73-78, 85-101 |
| src/posting/jumper.py                                 |       34 |        1 |     97% |        55 |
| src/posting/locations.py                              |       18 |        3 |     83% |26, 31, 35 |
| src/posting/messages.py                               |        6 |        0 |    100% |           |
| src/posting/request\_headers.py                       |        8 |        0 |    100% |           |
| src/posting/save\_request.py                          |        9 |        0 |    100% |           |
| src/posting/scripts.py                                |       78 |       17 |     78% |16, 57, 77-79, 83-84, 117-118, 145, 150, 176, 182, 211-214 |
| src/posting/suggesters.py                             |        1 |        1 |      0% |         2 |
| src/posting/themes.py                                 |      176 |        7 |     96% |236, 335-336, 345-346, 350-351 |
| src/posting/tuple\_to\_multidict.py                   |       10 |        0 |    100% |           |
| src/posting/types.py                                  |        3 |        0 |    100% |           |
| src/posting/urls.py                                   |        5 |        0 |    100% |           |
| src/posting/user\_host.py                             |       11 |        2 |     82% |     14-15 |
| src/posting/variables.py                              |       66 |        5 |     92% |66-67, 119, 151, 154 |
| src/posting/version.py                                |        2 |        0 |    100% |           |
| src/posting/widgets/\_\_init\_\_.py                   |        0 |        0 |    100% |           |
| src/posting/widgets/center\_middle.py                 |        3 |        0 |    100% |           |
| src/posting/widgets/collection/browser.py             |      314 |       79 |     75% |121, 131, 140, 172, 267, 275, 289-290, 299, 307, 329, 343-345, 362, 366, 371, 435-441, 447-455, 458-483, 490-497, 500-526, 539-540, 654-662 |
| src/posting/widgets/collection/new\_request\_modal.py |      149 |        6 |     96% |18, 191, 271, 276, 285-289 |
| src/posting/widgets/confirmation.py                   |       39 |       21 |     46% |51-57, 60-64, 67-72, 76, 80, 84 |
| src/posting/widgets/datatable.py                      |      173 |       37 |     79% |67, 98, 111, 116, 119-122, 143, 161, 167, 171, 174-186, 194-199, 206-207, 212-213, 225-226, 254-256 |
| src/posting/widgets/input.py                          |       17 |        1 |     94% |        20 |
| src/posting/widgets/key\_value.py                     |      202 |       36 |     82% |28, 112-120, 170-173, 184-186, 194-195, 210, 214, 226, 233, 250, 264-271, 274-275, 283-285, 297, 316-327 |
| src/posting/widgets/request/\_\_init\_\_.py           |        0 |        0 |    100% |           |
| src/posting/widgets/request/form\_editor.py           |       31 |        0 |    100% |           |
| src/posting/widgets/request/header\_editor.py         |       67 |        6 |     91% |   284-289 |
| src/posting/widgets/request/method\_selection.py      |       29 |        2 |     93% |    75, 86 |
| src/posting/widgets/request/query\_editor.py          |       33 |        4 |     88% |32-33, 38-39 |
| src/posting/widgets/request/request\_auth.py          |      143 |       57 |     60% |24, 52, 96-99, 102, 105, 178-190, 197-220, 230-233, 238-260, 271-274 |
| src/posting/widgets/request/request\_body.py          |       23 |        0 |    100% |           |
| src/posting/widgets/request/request\_editor.py        |       74 |        3 |     96% |22, 95, 108 |
| src/posting/widgets/request/request\_metadata.py      |       40 |        3 |     92% |     17-19 |
| src/posting/widgets/request/request\_options.py       |       79 |       16 |     80% |116-121, 126, 133-134, 139-146 |
| src/posting/widgets/request/request\_scripts.py       |       81 |       32 |     60% |53-71, 81-110, 118, 126, 226-229 |
| src/posting/widgets/request/url\_bar.py               |      187 |       24 |     87% |36-37, 73, 106-109, 149-150, 154, 160-163, 170, 187, 206-207, 213-214, 229-232, 246, 280-281 |
| src/posting/widgets/response/cookies\_table.py        |       27 |        0 |    100% |           |
| src/posting/widgets/response/response\_area.py        |      110 |       21 |     81% |60-61, 86-87, 108-111, 117, 137, 141, 149-157, 161-164 |
| src/posting/widgets/response/response\_body.py        |       12 |        0 |    100% |           |
| src/posting/widgets/response/response\_headers.py     |        9 |        0 |    100% |           |
| src/posting/widgets/response/response\_trace.py       |       38 |        5 |     87% | 63, 75-78 |
| src/posting/widgets/response/script\_output.py        |       63 |        1 |     98% |        80 |
| src/posting/widgets/rich\_log.py                      |       29 |        0 |    100% |           |
| src/posting/widgets/select.py                         |       18 |        5 |     72% |17-20, 24, 30 |
| src/posting/widgets/tabbed\_content.py                |       12 |        6 |     50% |14-16, 19-21 |
| src/posting/widgets/text\_area.py                     |      371 |      183 |     51% |86-88, 134, 172-173, 193-202, 205-228, 231-262, 268, 271-275, 280-285, 287-334, 339-344, 349-360, 462, 467, 470-475, 478, 481, 484, 487, 490, 493, 496, 499, 502-525, 528, 531, 535-549, 553-560, 564-571, 575, 578-583, 588-599, 639, 651, 656-661 |
| src/posting/widgets/tree.py                           |       26 |        8 |     69% |35-40, 54-55 |
| src/posting/widgets/variable\_autocomplete.py         |       41 |        7 |     83% | 46, 71-77 |
| src/posting/widgets/variable\_input.py                |       25 |        0 |    100% |           |
| src/posting/xresources.py                             |       24 |       17 |     29% |     22-45 |
| src/posting/yaml.py                                   |       14 |        4 |     71% |6-7, 12-15 |
|                                             **TOTAL** | **4846** | **1141** | **76%** |           |


## Setup coverage badge

Below are examples of the badges you can use in your main branch `README` file.

### Direct image

[![Coverage badge](https://raw.githubusercontent.com/darrenburns/posting/python-coverage-comment-action-data/badge.svg)](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

This is the one to use if your repository is private or if you don't want to customize anything.

### [Shields.io](https://shields.io) Json Endpoint

[![Coverage badge](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/darrenburns/posting/python-coverage-comment-action-data/endpoint.json)](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

Using this one will allow you to [customize](https://shields.io/endpoint) the look of your badge.
It won't work with private repositories. It won't be refreshed more than once per five minutes.

### [Shields.io](https://shields.io) Dynamic Badge

[![Coverage badge](https://img.shields.io/badge/dynamic/json?color=brightgreen&label=coverage&query=%24.message&url=https%3A%2F%2Fraw.githubusercontent.com%2Fdarrenburns%2Fposting%2Fpython-coverage-comment-action-data%2Fendpoint.json)](https://htmlpreview.github.io/?https://github.com/darrenburns/posting/blob/python-coverage-comment-action-data/htmlcov/index.html)

This one will always be the same color. It won't work for private repos. I'm not even sure why we included it.

## What is that?

This branch is part of the
[python-coverage-comment-action](https://github.com/marketplace/actions/python-coverage-comment)
GitHub Action. All the files in this branch are automatically generated and may be
overwritten at any moment.
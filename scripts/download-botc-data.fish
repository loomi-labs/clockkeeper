#!/usr/bin/env fish
#
# Download Blood on the Clocktower game data and character icons
# from ThePandemoniumInstitute/botc-release
#

set repo ThePandemoniumInstitute/botc-release
set branch main
set base_url "https://raw.githubusercontent.com/$repo/$branch"
set data_dir (status dirname)/../data
set editions tb bmr snv carousel fabled loric

# Create directories
mkdir -p $data_dir/characters
for edition in $editions
    mkdir -p $data_dir/characters/$edition
end

# Download data files
echo "Downloading data files..."
for file in roles.json nightsheet.json jinxes.json
    echo "  $file"
    curl -sL "$base_url/resources/data/$file" -o "$data_dir/$file"
end

echo "  script-schema.json"
curl -sL "$base_url/script-schema.json" -o "$data_dir/script-schema.json"

# Download character icons per edition
for edition in $editions
    echo "Downloading $edition icons..."
    set files (gh api "repos/$repo/contents/resources/characters/$edition" --jq '.[].name')
    set count (count $files)
    set i 0
    for file in $files
        set i (math $i + 1)
        printf "  [%d/%d] %s\n" $i $count $file
        curl -sL "$base_url/resources/characters/$edition/$file" -o "$data_dir/characters/$edition/$file"
    end
end

echo "Done. Data saved to $data_dir"

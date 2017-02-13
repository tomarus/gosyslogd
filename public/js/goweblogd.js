var WebLogd = {
    Run: function() {
        this.top = new TopStats();
        this.log = new LogTail($("#unhandled"), "00000000000000000000000000000000");
    },

    SetTail: function(md5) {
        this.log.ResetMd5(md5);
    }
}

function TopStats(obj) {
    this.Run();
}

TopStats.prototype = {
    top_timer_id: 0,

    Run: function() {
        var self = this
        this.top_refresh();

        $("#tagtable").tablesorter();
        $("#pritable").tablesorter();
        $("#hosttable").tablesorter();
        $("#tagtable").trigger("sorton",  [[[1,1]]]);
        $("#pritable").trigger("sorton",  [[[1,1]]]);
        $("#hosttable").trigger("sorton", [[[1,1]]]);

        top_timer_id = setInterval(function(){self.top_refresh()}, 5000)
    },

    top_refresh: function() {
        var self = this
        $.getJSON('/data/stats', function(data) {
            if (!data) {
                console.log("No data received.");
                return;
            }

            //console.log("top_refresh data"+JSON.stringify(data, null, 4));
            $.each(data.tags,  function(key, val) { self.top_addorupdaterow("tagtable",  "tag", key, val) });
            $.each(data.pri,   function(key, val) { self.top_addorupdaterow("pritable",  "pri", key, val) });
            $.each(data.hosts, function(key, val) { self.top_addorupdaterow("hosttable", "host", key, val) });

            $("#tagtable").trigger("updateAll");
            $("#pritable").trigger("updateAll");
            $("#hosttable").trigger("updateAll");
        });
    },

    top_addorupdaterow: function(table, url, key, val) {
        hkey = encodeURI(key)
        md5key = CryptoJS.MD5(key)
        h = "<tr><td data-id=\""+md5key+"\"><a href=\"javascript:WebLogd.SetTail('"+md5key+"')\">"+hkey+"</a></td><td>"+val+"</td></tr>";

        var found = 0
        $("#"+table+" tbody tr").find("td").each(function(k,v){
            //var s = $(this).text()
            var s = $(this).attr("data-id")
            if ( s == md5key ) {
                $(this).parent().replaceWith(h);
                found++;
                return;
            }
        });

        // Append table row if it didn't exist and wasn't updated.
        if ( found == 0 ) {
            $("#"+table+" tbody:last").append(h);
        }
    }
};

function LogTail(obj,md5) {
    this.obj = obj;
    this.md5 = md5;
    this.Refill();
    this.Tail();

    var me = this
    $("#tailopt").click(function() {
        if ( $("#tailopt").is(":checked") ) {
            me.Refill();
            me.Tail();
        } else {
            me.sock.close();
        }
    });
}

LogTail.prototype = {
    ResetMd5: function(md5) {
        this.md5 = md5;
        this.sock.close();
        this.Refill();
        this.Tail();
    },

    Tail: function() {
        var url = "ws://"+location.hostname+":5145/stream?md5="+this.md5;
        this.sock = new WebSocket(url);
        console.log("Connecting to: "+url);

        var self = this;

        this.sock.onmessage = function (event) {
            var msg;
            try {
                msg = $.parseJSON(event.data)
            } catch(e) {
                console.log("Unhandled websocket message: "+event.data)
                return;
            }
            self.obj.find('tbody').prepend('<tr><td>'+atob(msg.Raw)+'</td></tr>');
            self.obj.find('tbody tr:first td').effect("highlight", {'color': '#ff7777'}, 3000);
            self.limitTable();
        };
        this.sock.onopen = function () {
            console.log("Connected");
        };
        this.sock.onclose = function () {
            console.log("Closed");
        };
    },

    limitTable: function() {
        if ( this.obj.find('tbody').find("tr").length > 20 ) {
            this.obj.find('tbody').find("tr").last().remove();
        }
    },

    Refill: function() {
        var obj = this.obj;
        $.getJSON('/log?md5='+this.md5+'&max=20', function(data) {
            if (!data) {
                console.log("No data received.");
                return;
            }

            obj.find("tbody").find("tr").remove();
            $.each(data, function(key, val) {
                h = "<tr><td>"+atob(val.Raw)+"</td></tr>";
                obj.find("tbody:last").append(h);
            });
        });
    },
};

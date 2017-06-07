var MESSAGE_FORMATTER = [
    function(msg) {
        var style = FILTER.i ? ' style="display:none"' : '';
        return '<div class="_i"'+style+'>' + msg + '</div>'
    },
    function(msg) {
        var style = FILTER.e ? ' style="display:none"' : '';
        return '<div class="_e"'+style+'>' + msg + '</div>'
    },
    function(msg) {
        $('#flash').append('<div class="alert alert-danger alert-dismissible" role="alert">\
                                    <button type="button" class="close" data-dismiss="alert" aria-label="Close"><span aria-hidden="true">&times;</span></button>\
                                    <strong>ERRO!</strong> ' + msg + '\
                                  </div>');
    },
    function(msg) {
        _close();
    },
    function(msg) {
        $('#flash').append('<div class="alert alert-info alert-dismissible" role="alert">\
                                                     <button type="button" class="close" data-dismiss="alert" aria-label="Close"><span aria-hidden="true">&times;</span></button>\
                                                     ' + msg + '</div>');
    },
    function(msg) {
        $('#fileInfo').show();
        var data = '';
        eval('msg = ' + msg + ";");
        delete msg["Is"];
        for (var k in msg) {
            data += '<p><b>' + k + '</b>: ' + msg[k] + '</p>';
        }
        $('#fileInfoModal .modal-body').html(data);
    },
    function(msg) {
        if (msg) {
            $('#follow').show()
        } else {
            $('#follow,#fileInfo').hide()
        }
    }
];

var FILTER = {
    i: false,
    e: false
};

function _close() {
    window.theSocket.close();
    $('#start').show();
    $('#stop').hide();
};

function _start() {
    if (window.theSocket) return;

    var ws, $status = $('#status');
    if (window.WebSocket === undefined) {
        $("#log").append("Your browser does not support WebSockets");
        return;
    } else {
        var socket = window.theSocket = new WebSocket(window.WS_URL),
            container = $("#log");
        socket.onopen = function() {
            $status.html("Conexão iniciada.");
            $('#start').hide();
            $('#stop').show();
        };
        socket.onmessage = function (e) {
            var mtype = parseInt(e.data.substring(0, 2), 16);
            msg = MESSAGE_FORMATTER[mtype](e.data.substring(2));

            if (msg)
                container.append(msg);
        };
        socket.onclose = function () {
            $status.html("Conexão encerrada");
            window.theSocket = null;
            $('#stop').hide();
            $('#start').show();
            $('#follow').hide()
        };
    }

    $('#start').hide();
};

$(function () {
    var $log = $("#log");
    $('#stop').hide().click(function() {
        if (window.theSocket) _close();
    });

    $('#start').click(function() {
        if (!window.theSocket) _start();
    });

    $('#logClean').click(function() {
        $log.html('');
    });

    $('#filter').children().click(function() {
        var $this = $(this);
        $this.siblings().removeClass('active');
        $this.addClass('active');
        var filterValue = $this.data('filter');

        if(filterValue == '*') {
            FILTER.e = FILTER.i = false;
            $log.children().show();
        } else if(filterValue == 'e') {
            FILTER.i = true;
            FILTER.e = false;
            $log.children('._e').show();
            $log.children('._i').hide();
        } else if(filterValue == 'i') {
            FILTER.e = true;
            FILTER.i = false;
            $log.children('._i').show();
            $log.children('._e').hide();
        }
    });

    $('#fileInfo').popover({});
});
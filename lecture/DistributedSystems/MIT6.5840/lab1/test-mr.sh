#!/usr/bin/env bash

if [[ "$OSTYPE" = "darwin"* ]]
then
  if go version | grep 'go1.17.[012345]'
  then
    # -race with plug-ins on x86 MacOS 12 with
    # go1.17 before 1.17.6 sometimes crash.
    export RACE=""
    echo '*** Turning off -race since it may not work on a Mac'
    echo '    with ' `go version`
  fi
fi

ISQUIET=$1
maybe_quiet() {
    if [ "$ISQUIET" == "quiet" ]; then
      "$@" > /dev/null 2>&1
    else
      "$@"
    fi
}


TIMEOUT=timeout
TIMEOUT2=""
if timeout 2s sleep 1 > /dev/null 2>&1
then
  :
else
  if gtimeout 2s sleep 1 > /dev/null 2>&1
  then
    TIMEOUT=gtimeout
  else
    # no timeout command
    TIMEOUT=
    echo '*** Cannot find timeout command; proceeding without timeouts.'
  fi
fi
if [ "$TIMEOUT" != "" ]
then
  TIMEOUT2=$TIMEOUT
  TIMEOUT2+=" -k 2s 120s "
  TIMEOUT+=" -k 2s 45s "
fi

if [ "$2" == "" ]; then
  make -e
fi

LIB_DIR=output/lib
BIN_DIR=output/bin
DATA_DIR=output/data
export LOCAL_PATH=output/workspace
OUTPUT_PATH=$LOCAL_PATH/result

failed_any=0

#########################################################
rm -f output/mr-*
# clean workspace directory and recreate
rm -rf $LOCAL_PATH
mkdir $LOCAL_PATH

# first word-count

# generate the correct output
$BIN_DIR/mrsequential $LIB_DIR/wc.so $DATA_DIR/pg*txt || exit 1
sort mr-out-0 > output/mr-correct-wc.txt
rm -f mr-out*

echo '***' Starting wc test.

maybe_quiet $TIMEOUT $BIN_DIR/coordinator $DATA_DIR/pg*txt &
pid=$!

# give the coordinator time to create the sockets.
sleep 1

# start multiple workers.
(maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/wc.so) &
(maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/wc.so) &
(maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/wc.so) &

# wait for the coordinator to exit.
wait $pid

# since workers are required to exit when a job is completely finished,
# and not before, that means the job has finished.
sort $OUTPUT_PATH/mr-out* | grep . > output/mr-wc-all
if cmp output/mr-wc-all output/mr-correct-wc.txt
then
  echo '---' wc test: PASS
else
  echo '---' wc output is not the same as mr-correct-wc.txt
  echo '---' wc test: FAIL
  failed_any=1
fi

# wait for remaining workers and coordinator to exit.
wait

#########################################################
# now indexer
rm -f output/mr-*
# clean workspace directory and recreate
rm -rf $LOCAL_PATH
mkdir $LOCAL_PATH

# generate the correct output
$BIN_DIR/mrsequential $LIB_DIR/indexer.so $DATA_DIR/pg*txt || exit 1
sort mr-out-0 > output/mr-correct-indexer.txt
rm -f mr-out*

echo '***' Starting indexer test.

maybe_quiet $TIMEOUT $BIN_DIR/coordinator $DATA_DIR/pg*txt &
sleep 1

# start multiple workers
maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/indexer.so &
maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/indexer.so

sort $OUTPUT_PATH/mr-out* | grep . > output/mr-indexer-all
if cmp output/mr-indexer-all output/mr-correct-indexer.txt
then
  echo '---' indexer test: PASS
else
  echo '---' indexer output is not the same as mr-correct-indexer.txt
  echo '---' indexer test: FAIL
  failed_any=1
fi

wait

#########################################################
echo '***' Starting map parallelism test.

rm -f output/mr-*
# clean workspace directory and recreate
rm -rf $LOCAL_PATH
mkdir $LOCAL_PATH

maybe_quiet $TIMEOUT $BIN_DIR/coordinator $DATA_DIR/pg*txt &
sleep 1

maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/mtiming.so &
maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/mtiming.so
cp $OUTPUT_PATH/mr-out* output

NT=`cat output/mr-out* | grep '^times-' | wc -l | sed 's/ //g'`
if [ "$NT" != "2" ]
then
  echo '---' saw "$NT" workers rather than 2
  echo '---' map parallelism test: FAIL
  failed_any=1
fi

if cat output/mr-out* | grep '^parallel.* 2' > /dev/null
then
  echo '---' map parallelism test: PASS
else
  echo '---' map workers did not run in parallel
  echo '---' map parallelism test: FAIL
  failed_any=1
fi

wait

#########################################################
echo '***' Starting reduce parallelism test.

rm -f output/mr-*
# clean workspace directory and recreate
rm -rf $LOCAL_PATH
mkdir $LOCAL_PATH

maybe_quiet $TIMEOUT $BIN_DIR/coordinator $DATA_DIR/pg*txt &
sleep 1

maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/rtiming.so  &
maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/rtiming.so
cp $OUTPUT_PATH/mr-out* output

NT=`cat output/mr-out* | grep '^[a-z] 2' | wc -l | sed 's/ //g'`
if [ "$NT" -lt "2" ]
then
  echo '---' too few parallel reduces.
  echo '---' reduce parallelism test: FAIL
  failed_any=1
else
  echo '---' reduce parallelism test: PASS
fi

wait

#########################################################
echo '***' Starting job count test.

rm -f output/mr-*
# clean workspace directory and recreate
rm -rf $LOCAL_PATH
mkdir $LOCAL_PATH

maybe_quiet $TIMEOUT $BIN_DIR/coordinator $DATA_DIR/pg*txt  &
sleep 1

maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/jobcount.so &
maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/jobcount.so
maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/jobcount.so &
maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/jobcount.so
cp $OUTPUT_PATH/mr-out* output

NT=`cat output/mr-out* | awk '{print $2}'`
if [ "$NT" -eq "8" ]
then
  echo '---' job count test: PASS
else
  echo '---' map jobs ran incorrect number of times "($NT != 8)"
  echo '---' job count test: FAIL
  failed_any=1
fi

wait

#########################################################
# test whether any worker or coordinator exits before the
# task has completed (i.e., all output files have been finalized)
rm -f output/mr-*
# clean workspace directory and recreate
rm -rf $LOCAL_PATH
mkdir $LOCAL_PATH
rm -f mr-worker-jobcount-*


echo '***' Starting early exit test.

DF=anydone$$
rm -f $DF

(maybe_quiet $TIMEOUT $BIN_DIR/coordinator $DATA_DIR/pg*txt; touch $DF) &

# give the coordinator time to create the sockets.
sleep 1

# start multiple workers.
(maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/early_exit.so; touch $DF) &
(maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/early_exit.so; touch $DF) &
(maybe_quiet $TIMEOUT $BIN_DIR/worker $LIB_DIR/early_exit.so; touch $DF) &

# wait for any of the coord or workers to exit.
# `jobs` ensures that any completed old processes from other tests
# are not waited upon.
jobs &> /dev/null
if [[ "$OSTYPE" = "darwin"* ]]
then
  # bash on the Mac doesn't have wait -n
  while [ ! -e $DF ]
  do
    sleep 0.2
  done
else
  # the -n causes wait to wait for just one child process,
  # rather than waiting for all to finish.
  wait -n
fi

rm -f $DF

# a process has exited. this means that the output should be finalized
# otherwise, either a worker or the coordinator exited early
sort $OUTPUT_PATH/mr-out* | grep . > output/mr-wc-all-initial

# wait for remaining workers and coordinator to exit.
wait

# compare initial and final outputs
sort $OUTPUT_PATH/mr-out* | grep . > output/mr-wc-all-final
if cmp output/mr-wc-all-final output/mr-wc-all-initial
then
  echo '---' early exit test: PASS
else
  echo '---' output changed after first worker exited
  echo '---' early exit test: FAIL
  failed_any=1
fi

#########################################################
echo '***' Starting crash test.

rm -f output/mr-*
# clean workspace directory and recreate
rm -rf $LOCAL_PATH
mkdir $LOCAL_PATH

# generate the correct output
$BIN_DIR/mrsequential $LIB_DIR/nocrash.so $DATA_DIR/pg*txt || exit 1
sort mr-out-0 > output/mr-correct-crash.txt
rm -f mr-out*

rm -f mr-done
((maybe_quiet $TIMEOUT2 $BIN_DIR/coordinator $DATA_DIR/pg*txt); touch mr-done ) &
sleep 1

# start multiple workers
maybe_quiet $TIMEOUT2 $BIN_DIR/worker $LIB_DIR/crash.so &

# mimic rpc.go's coordinatorSock()
SOCKNAME=/var/tmp/5840-mr-`id -u`

( while [ -e $SOCKNAME -a ! -f mr-done ]
  do
    maybe_quiet $TIMEOUT2 $BIN_DIR/worker $LIB_DIR/crash.so
    sleep 1
  done ) &

( while [ -e $SOCKNAME -a ! -f mr-done ]
  do
    maybe_quiet $TIMEOUT2 $BIN_DIR/worker $LIB_DIR/crash.so
    sleep 1
  done ) &

while [ -e $SOCKNAME -a ! -f mr-done ]
do
  maybe_quiet $TIMEOUT2 $BIN_DIR/worker $LIB_DIR/crash.so
  sleep 1
done

wait

rm $SOCKNAME
sort $OUTPUT_PATH/mr-out* | grep . > output/mr-crash-all
if cmp output/mr-crash-all output/mr-correct-crash.txt
then
  echo '---' crash test: PASS
else
  echo '---' crash output is not the same as mr-correct-crash.txt
  echo '---' crash test: FAIL
  failed_any=1
fi

#########################################################
if [ $failed_any -eq 0 ]; then
    echo '***' PASSED ALL TESTS
else
    echo '***' FAILED SOME TESTS
    exit 1
fi
